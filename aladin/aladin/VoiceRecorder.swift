//
//  VoiceRecorder.swift
//  aladin
//

import AVFoundation
import Combine
import Foundation

@MainActor
final class VoiceRecorder: NSObject, ObservableObject {
    enum PermissionState {
        case unknown
        case granted
        case denied
    }

    @Published private(set) var isRecording = false
    @Published private(set) var recordedFileURL: URL?
    @Published private(set) var permissionState: PermissionState = .unknown
    @Published private(set) var errorMessage: String?
    @Published private(set) var inputLevel: CGFloat = 0
    @Published private(set) var elapsedDuration: TimeInterval = 0
    @Published private(set) var recordedDuration: TimeInterval = 0
    @Published private(set) var isPlaying = false
    @Published private(set) var isPlaybackPaused = false

    var statusText: String {
        if isRecording {
            return "Recording in progress"
        }
        if isPlaying {
            return "Playing preview"
        }
        if isPlaybackPaused {
            return "Preview paused"
        }
        if recordedFileURL != nil {
            return "Recording ready to save"
        }
        switch permissionState {
        case .unknown:
            return "Microphone permission required"
        case .granted:
            return "Ready to record"
        case .denied:
            return "Microphone access denied"
        }
    }

    var statusSymbol: String {
        if isRecording {
            return "record.circle.fill"
        }
        if isPlaying {
            return "play.circle.fill"
        }
        if isPlaybackPaused {
            return "pause.circle.fill"
        }
        if recordedFileURL != nil {
            return "checkmark.circle"
        }
        switch permissionState {
        case .denied:
            return "mic.slash"
        case .unknown, .granted:
            return "mic"
        }
    }

    private var recorder: AVAudioRecorder?
    private var player: AVAudioPlayer?
    private var currentFileURL: URL?
    private var meterTask: Task<Void, Never>?
    private var playbackTask: Task<Void, Never>?

    override init() {
        super.init()
        permissionState = Self.currentPermissionState()
    }

    func startRecording() async {
        errorMessage = nil

        let granted = await ensurePermission()
        guard granted else {
            errorMessage = "Microphone access was not granted."
            return
        }

        do {
            discardRecording()

            let fileURL = try makeRecordingURL()
            let settings: [String: Any] = [
                AVFormatIDKey: kAudioFormatMPEG4AAC,
                AVSampleRateKey: 44_100,
                AVNumberOfChannelsKey: 1,
                AVEncoderAudioQualityKey: AVAudioQuality.high.rawValue
            ]

            let recorder = try AVAudioRecorder(url: fileURL, settings: settings)
            recorder.delegate = self
            recorder.isMeteringEnabled = true
            recorder.prepareToRecord()

            guard recorder.record() else {
                throw RecorderError.startFailed
            }

            self.recorder = recorder
            self.currentFileURL = fileURL
            self.recordedFileURL = nil
            self.isRecording = true
            self.inputLevel = 0
            self.elapsedDuration = 0
            self.recordedDuration = 0
            self.isPlaybackPaused = false
            self.startMetering()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func stopRecording() {
        guard let recorder else { return }
        recorder.stop()
        self.recorder = nil
        isRecording = false
        stopMetering()
        recordedDuration = recorder.currentTime
        elapsedDuration = 0

        if let currentFileURL, FileManager.default.fileExists(atPath: currentFileURL.path) {
            recordedFileURL = currentFileURL
        }
    }

    func togglePlayback() {
        if isPlaying {
            pausePlayback()
        } else if isPlaybackPaused {
            resumePlayback()
        } else {
            startPlayback()
        }
    }

    func seek(to time: TimeInterval) {
        let clampedTime = min(max(time, 0), recordedDuration)
        elapsedDuration = clampedTime

        guard let player else { return }
        player.currentTime = clampedTime

        if clampedTime >= recordedDuration {
            if isPlaying {
                player.pause()
                stopPlaybackTimer()
            }
            isPlaying = false
            isPlaybackPaused = true
        }
    }

    func discardRecording() {
        recorder?.stop()
        recorder = nil
        isRecording = false
        stopMetering(resetElapsedTime: true)
        stopPlayback(resetElapsedTime: true, resetToStart: true)

        let fileURL = recordedFileURL ?? currentFileURL
        recordedFileURL = nil
        currentFileURL = nil
        recordedDuration = 0

        if let fileURL {
            try? FileManager.default.removeItem(at: fileURL)
        }
    }

    private func startMetering() {
        meterTask?.cancel()
        meterTask = Task { [weak self] in
            while !Task.isCancelled {
                guard let self else { return }
                if let recorder = self.recorder, recorder.isRecording {
                    recorder.updateMeters()
                    let averagePower = recorder.averagePower(forChannel: 0)
                    let normalizedLevel = Self.normalizePowerLevel(averagePower)
                    self.inputLevel = (self.inputLevel * 0.35) + (normalizedLevel * 0.65)
                    self.elapsedDuration = recorder.currentTime
                } else {
                    self.inputLevel = 0
                }

                try? await Task.sleep(for: .milliseconds(50))
            }
        }
    }

    private func stopMetering(resetElapsedTime: Bool = false) {
        meterTask?.cancel()
        meterTask = nil
        inputLevel = 0
        if resetElapsedTime {
            elapsedDuration = 0
        }
    }

    private func startPlayback() {
        guard let url = recordedFileURL else { return }

        do {
            stopPlayback(resetElapsedTime: false, resetToStart: false)
            let player = try AVAudioPlayer(contentsOf: url)
            player.delegate = self
            player.prepareToPlay()
            if elapsedDuration >= recordedDuration, recordedDuration > 0 {
                elapsedDuration = 0
            }
            player.currentTime = elapsedDuration

            guard player.play() else {
                throw RecorderError.playbackFailed
            }

            self.player = player
            self.recordedDuration = max(recordedDuration, player.duration)
            self.elapsedDuration = player.currentTime
            self.isPlaying = true
            self.isPlaybackPaused = false
            startPlaybackTimer()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func resumePlayback() {
        guard let player else {
            startPlayback()
            return
        }
        if elapsedDuration >= recordedDuration, recordedDuration > 0 {
            elapsedDuration = 0
            player.currentTime = 0
        }
        guard player.play() else {
            errorMessage = RecorderError.playbackFailed.localizedDescription
            return
        }
        isPlaying = true
        isPlaybackPaused = false
        startPlaybackTimer()
    }

    private func pausePlayback() {
        guard let player else { return }
        player.pause()
        elapsedDuration = player.currentTime
        isPlaying = false
        isPlaybackPaused = true
        stopPlaybackTimer()
    }

    private func stopPlayback(resetElapsedTime: Bool, resetToStart: Bool) {
        stopPlaybackTimer()
        player?.stop()
        player = nil
        isPlaying = false
        isPlaybackPaused = false
        if resetElapsedTime {
            elapsedDuration = 0
        } else if resetToStart {
            elapsedDuration = 0
        } else if recordedFileURL != nil, elapsedDuration > recordedDuration {
            elapsedDuration = recordedDuration
        }
    }

    private func startPlaybackTimer() {
        stopPlaybackTimer()
        playbackTask = Task { [weak self] in
            while !Task.isCancelled {
                guard let self else { return }
                if let player = self.player, player.isPlaying {
                    self.elapsedDuration = player.currentTime
                } else {
                    return
                }

                try? await Task.sleep(for: .milliseconds(50))
            }
        }
    }

    private func stopPlaybackTimer() {
        playbackTask?.cancel()
        playbackTask = nil
    }

    private func ensurePermission() async -> Bool {
        switch Self.currentPermissionState() {
        case .granted:
            permissionState = .granted
            return true
        case .denied:
            permissionState = .denied
            return false
        case .unknown:
            let granted = await withCheckedContinuation { continuation in
                AVCaptureDevice.requestAccess(for: .audio) { allowed in
                    continuation.resume(returning: allowed)
                }
            }
            permissionState = granted ? .granted : .denied
            return granted
        }
    }

    private func makeRecordingURL() throws -> URL {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("voice-notes", isDirectory: true)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true, attributes: nil)
        return directory.appendingPathComponent("voice-note-\(UUID().uuidString).m4a")
    }

    private static func currentPermissionState() -> PermissionState {
        switch AVCaptureDevice.authorizationStatus(for: .audio) {
        case .authorized:
            return .granted
        case .denied, .restricted:
            return .denied
        case .notDetermined:
            return .unknown
        @unknown default:
            return .unknown
        }
    }

    private static func normalizePowerLevel(_ power: Float) -> CGFloat {
        guard power.isFinite else { return 0 }
        let minPower: Float = -50
        if power <= minPower { return 0 }
        if power >= 0 { return 1 }
        return CGFloat((power - minPower) / -minPower)
    }
}

extension VoiceRecorder: AVAudioRecorderDelegate {
    nonisolated func audioRecorderEncodeErrorDidOccur(_ recorder: AVAudioRecorder, error: Error?) {
        guard let error else { return }
        Task { @MainActor in
            self.errorMessage = error.localizedDescription
            self.isRecording = false
            self.stopMetering()
        }
    }
}

extension VoiceRecorder: AVAudioPlayerDelegate {
    nonisolated func audioPlayerDidFinishPlaying(_ player: AVAudioPlayer, successfully flag: Bool) {
        Task { @MainActor in
            self.stopPlayback(resetElapsedTime: false, resetToStart: false)
        }
    }

    nonisolated func audioPlayerDecodeErrorDidOccur(_ player: AVAudioPlayer, error: Error?) {
        guard let error else { return }
        Task { @MainActor in
            self.errorMessage = error.localizedDescription
            self.stopPlayback(resetElapsedTime: false, resetToStart: false)
        }
    }
}

private enum RecorderError: LocalizedError {
    case startFailed
    case playbackFailed

    var errorDescription: String? {
        switch self {
        case .startFailed:
            return "The microphone recording session could not be started."
        case .playbackFailed:
            return "The recorded audio preview could not be played."
        }
    }
}
