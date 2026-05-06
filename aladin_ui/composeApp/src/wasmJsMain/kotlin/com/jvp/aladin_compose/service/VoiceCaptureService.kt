package com.jvp.aladin_compose.service

interface VoiceCaptureService {
    fun start(
        onLevel: (Float) -> Unit,
        onStarted: () -> Unit,
        onError: (String) -> Unit,
    ): VoiceCaptureSession
}

interface VoiceCaptureSession {
    fun stop(
        onComplete: (VoiceCaptureResult) -> Unit,
        onError: (String) -> Unit,
    )

    fun cancel()
}

data class VoiceCaptureResult(
    val base64Data: String,
    val contentType: String?,
    val durationMs: Long,
)

interface VoicePlaybackHandle {
    fun play()

    fun pause()

    fun seek(positionMs: Long)

    fun dispose()
}

fun createVoicePlaybackHandle(
    url: String,
    onPositionChanged: (Long) -> Unit,
    onEnded: () -> Unit,
    onError: (String) -> Unit,
): VoicePlaybackHandle {
    return BrowserVoicePlaybackHandle(
        createBrowserVoicePlayback(
            url = url,
            onPositionChanged = { positionMs -> onPositionChanged(positionMs.toLong()) },
            onEnded = onEnded,
            onError = onError,
        )
    )
}

class BrowserVoiceCaptureService : VoiceCaptureService {
    override fun start(
        onLevel: (Float) -> Unit,
        onStarted: () -> Unit,
        onError: (String) -> Unit,
    ): VoiceCaptureSession {
        val session =
            startBrowserVoiceCapture(
                onLevel = onLevel,
                onStarted = onStarted,
                onError = onError,
            )
        return BrowserVoiceCaptureSession(session)
    }
}

private class BrowserVoiceCaptureSession(private val session: BrowserVoiceCaptureHandle) :
    VoiceCaptureSession {
    override fun stop(
        onComplete: (VoiceCaptureResult) -> Unit,
        onError: (String) -> Unit,
    ) {
        session.stop(
            { result ->
                onComplete(
                    VoiceCaptureResult(
                        base64Data = result.base64Data,
                        contentType = result.contentType,
                        durationMs = result.durationMs.toLong(),
                    )
                )
            },
            onError,
        )
    }

    override fun cancel() {
        session.cancel()
    }
}

private class BrowserVoicePlaybackHandle(private val handle: BrowserVoicePlaybackInteropHandle) :
    VoicePlaybackHandle {
    override fun play() {
        handle.play()
    }

    override fun pause() {
        handle.pause()
    }

    override fun seek(positionMs: Long) {
        handle.seek(positionMs.toDouble())
    }

    override fun dispose() {
        handle.dispose()
    }
}

@OptIn(ExperimentalWasmJsInterop::class)
private external interface BrowserVoiceCaptureResult : JsAny {
    val base64Data: String
    val contentType: String?
    val durationMs: Double
}

@OptIn(ExperimentalWasmJsInterop::class)
private external interface BrowserVoiceCaptureHandle : JsAny {
    fun stop(onComplete: (BrowserVoiceCaptureResult) -> Unit, onError: (String) -> Unit)

    fun cancel()
}

@OptIn(ExperimentalWasmJsInterop::class)
private external interface BrowserVoicePlaybackInteropHandle : JsAny {
    fun play()

    fun pause()

    fun seek(positionMs: Double)

    fun dispose()
}

@OptIn(ExperimentalWasmJsInterop::class)
@JsFun(
    """
    (onLevel, onStarted, onError) => {
      let stream = null;
      let recorder = null;
      let chunks = [];
      let audioContext = null;
      let analyser = null;
      let source = null;
      let rafId = null;
      let stopped = false;
      let startTime = Date.now();

      const cleanup = () => {
        if (rafId != null) cancelAnimationFrame(rafId);
        rafId = null;
        if (source) source.disconnect();
        source = null;
        if (audioContext) audioContext.close().catch(() => {});
        audioContext = null;
        if (stream) stream.getTracks().forEach((track) => track.stop());
        stream = null;
      };

      const tick = () => {
        if (stopped || !analyser) return;
        const data = new Uint8Array(analyser.fftSize);
        analyser.getByteTimeDomainData(data);
        let sum = 0;
        for (let i = 0; i < data.length; i++) {
          const centered = (data[i] - 128) / 128;
          sum += centered * centered;
        }
        const rms = Math.sqrt(sum / data.length);
        onLevel(Math.min(1, rms * 3.8));
        rafId = requestAnimationFrame(tick);
      };

      navigator.mediaDevices.getUserMedia({ audio: true })
        .then((nextStream) => {
          stream = nextStream;
          const mimeType = MediaRecorder.isTypeSupported("audio/webm;codecs=opus")
            ? "audio/webm;codecs=opus"
            : "audio/webm";
          recorder = new MediaRecorder(stream, { mimeType });
          recorder.ondataavailable = (event) => {
            if (event.data && event.data.size > 0) chunks.push(event.data);
          };
          audioContext = new AudioContext();
          analyser = audioContext.createAnalyser();
          analyser.fftSize = 256;
          source = audioContext.createMediaStreamSource(stream);
          source.connect(analyser);
          startTime = Date.now();
          recorder.start();
          onStarted();
          tick();
        })
        .catch((error) => {
          cleanup();
          onError(error && error.message ? String(error.message) : "Microphone access failed");
        });

      return {
        stop: (onComplete, stopError) => {
          if (stopped) return;
          stopped = true;
          if (!recorder) {
            cleanup();
            stopError("Recording did not start");
            return;
          }
          recorder.onstop = () => {
            const type = recorder.mimeType || "audio/webm";
            const blob = new Blob(chunks, { type });
            const reader = new FileReader();
            reader.onloadend = () => {
              cleanup();
              const value = String(reader.result || "");
              const base64Data = value.includes(",") ? value.substring(value.indexOf(",") + 1) : value;
              onComplete({
                base64Data,
                contentType: type,
                durationMs: Date.now() - startTime
              });
            };
            reader.onerror = () => {
              cleanup();
              stopError("Failed to read recording");
            };
            reader.readAsDataURL(blob);
          };
          recorder.stop();
        },
        cancel: () => {
          stopped = true;
          if (recorder && recorder.state !== "inactive") recorder.stop();
          cleanup();
        }
      };
    }
    """,
)
private external fun startBrowserVoiceCapture(
    onLevel: (Float) -> Unit,
    onStarted: () -> Unit,
    onError: (String) -> Unit,
): BrowserVoiceCaptureHandle

@OptIn(ExperimentalWasmJsInterop::class)
@JsFun(
    """
    (url, onPositionChanged, onEnded, onError) => {
      const audio = new Audio(url);
      audio.preload = "metadata";

      const notifyPosition = () => {
        if (!Number.isNaN(audio.currentTime)) {
          onPositionChanged(audio.currentTime * 1000);
        }
      };

      audio.ontimeupdate = notifyPosition;
      audio.onseeked = notifyPosition;
      audio.onended = () => {
        notifyPosition();
        onEnded();
      };
      audio.onerror = () => {
        onError("Failed to play recording preview");
      };

      return {
        play: () => {
          audio.play().catch((error) => {
            onError(error && error.message ? String(error.message) : "Failed to play recording preview");
          });
        },
        pause: () => {
          audio.pause();
          notifyPosition();
        },
        seek: (positionMs) => {
          audio.currentTime = Math.max(0, positionMs / 1000);
          notifyPosition();
        },
        dispose: () => {
          audio.pause();
          audio.removeAttribute("src");
          audio.load();
          audio.ontimeupdate = null;
          audio.onseeked = null;
          audio.onended = null;
          audio.onerror = null;
        }
      };
    }
    """,
)
private external fun createBrowserVoicePlayback(
    url: String,
    onPositionChanged: (Double) -> Unit,
    onEnded: () -> Unit,
    onError: (String) -> Unit,
): BrowserVoicePlaybackInteropHandle
