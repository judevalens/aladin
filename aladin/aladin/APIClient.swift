//
//  APIClient.swift
//  aladin
//

import Foundation
import UniformTypeIdentifiers

struct ArtifactSummary: Decodable, Identifiable, Equatable {
    let id: String
    let type: String
    let label: String
    let content: String
    let sourceURL: String?
    let childCount: Int
    let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id
        case type
        case label
        case content
        case sourceURL = "sourceUrl"
        case childCount
        case createdAt
    }
}

struct SourceSummary: Decodable, Identifiable {
    let id: String
    let name: String
    let type: String
    let syncMode: String
    let syncState: String
    let createdAt: String
    let lastSyncedAt: String?

    enum CodingKeys: String, CodingKey {
        case id
        case name
        case type
        case syncMode
        case syncState
        case createdAt
        case lastSyncedAt
    }
}

struct WorkerStatus: Decodable {
    struct Pipeline: Decodable {
        let enriched: Int
        let embedded: Int
        let promoted: Int
        let errors: Int
    }

    struct Queue: Decodable {
        let pending: Int
        let pendingPipeline: Int
        let deadJobs: Int

        enum CodingKeys: String, CodingKey {
            case pending
            case pendingPipeline
            case deadJobs
        }
    }

    let pipeline: Pipeline
    let queue: Queue
}

enum APIError: LocalizedError {
    case invalidURL
    case invalidResponse
    case server(String)

    var errorDescription: String? {
        switch self {
        case .invalidURL:
            return "Invalid API URL."
        case .invalidResponse:
            return "Unexpected API response."
        case .server(let message):
            return message
        }
    }
}

final class APIClient {
    private let baseURL: URL
    private let session: URLSession
    private let decoder: JSONDecoder

    nonisolated init(baseURL: URL = URL(string: "http://127.0.0.1:8000")!, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.session = session
        self.decoder = JSONDecoder()
    }

    func fetchArtifacts() async throws -> [ArtifactSummary] {
        try await get(path: "/api/artifacts/")
    }

    func fetchSources() async throws -> [SourceSummary] {
        try await get(path: "/api/sources/")
    }

    func fetchWorkerStatus() async throws -> WorkerStatus {
        try await get(path: "/api/worker/status")
    }

    func createArtifact(id: String? = nil, type: String, label: String, content: String, sourceURL: String? = nil) async throws {
        struct Payload: Encodable {
            let id: String?
            let type: String
            let label: String
            let content: String
            let sourceURL: String?

            enum CodingKeys: String, CodingKey {
                case id
                case type
                case label
                case content
                case sourceURL = "sourceUrl"
            }
        }

        let payload = Payload(id: id, type: type, label: label, content: content, sourceURL: sourceURL)
        _ = try await send(path: "/api/artifacts/", method: "POST", body: payload, response: EmptyResponse.self)
    }

    func createNote(label: String, content: String) async throws {
        struct Payload: Encodable {
            let label: String
            let content: String
        }

        _ = try await send(path: "/api/notes/", method: "POST", body: Payload(label: label, content: content), response: NoteCreateResponse.self)
    }

    func uploadAudio(fileURL: URL) async throws -> AudioUploadResponse {
        let data = try Data(contentsOf: fileURL)
        let boundary = "Boundary-\(UUID().uuidString)"
        var request = try request(path: "/api/audio/upload", method: "POST")
        request.setValue("multipart/form-data; boundary=\(boundary)", forHTTPHeaderField: "Content-Type")
        request.httpBody = makeMultipartBody(
            boundary: boundary,
            fieldName: "file",
            filename: fileURL.lastPathComponent,
            mimeType: mimeType(for: fileURL),
            fileData: data
        )

        let (responseData, response) = try await session.data(for: request)
        return try decode(responseData, response: response)
    }

    func uploadDocument(fileURL: URL) async throws {
        let data = try Data(contentsOf: fileURL)
        let boundary = "Boundary-\(UUID().uuidString)"
        var request = try request(path: "/api/documents/upload", method: "POST")
        request.setValue("multipart/form-data; boundary=\(boundary)", forHTTPHeaderField: "Content-Type")
        request.httpBody = makeMultipartBody(
            boundary: boundary,
            fieldName: "file",
            filename: fileURL.lastPathComponent,
            mimeType: "application/pdf",
            fileData: data
        )

        let (responseData, response) = try await session.data(for: request)
        _ = try decode(responseData, response: response) as DocumentUploadResponse
    }

    private func get<T: Decodable>(path: String) async throws -> T {
        let request = try request(path: path, method: "GET")
        let (data, response) = try await session.data(for: request)
        return try decode(data, response: response)
    }

    private func send<T: Decodable, Body: Encodable>(path: String, method: String, body: Body, response: T.Type) async throws -> T {
        var request = try request(path: path, method: method)
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(body)
        let (data, httpResponse) = try await session.data(for: request)
        return try decode(data, response: httpResponse)
    }

    private func request(path: String, method: String) throws -> URLRequest {
        guard let url = URL(string: path, relativeTo: baseURL) else {
            throw APIError.invalidURL
        }
        var request = URLRequest(url: url)
        request.httpMethod = method
        return request
    }

    private func decode<T: Decodable>(_ data: Data, response: URLResponse) throws -> T {
        guard let http = response as? HTTPURLResponse else {
            throw APIError.invalidResponse
        }
        guard (200..<300).contains(http.statusCode) else {
            if let body = try? decoder.decode([String: String].self, from: data),
               let message = body["error"] {
                throw APIError.server(message)
            }
            throw APIError.server("Request failed with status \(http.statusCode).")
        }
        return try decoder.decode(T.self, from: data)
    }

    private func makeMultipartBody(boundary: String, fieldName: String, filename: String, mimeType: String, fileData: Data) -> Data {
        var body = Data()
        let lineBreak = "\r\n"
        body.append("--\(boundary)\(lineBreak)".data(using: .utf8)!)
        body.append("Content-Disposition: form-data; name=\"\(fieldName)\"; filename=\"\(filename)\"\(lineBreak)".data(using: .utf8)!)
        body.append("Content-Type: \(mimeType)\(lineBreak)\(lineBreak)".data(using: .utf8)!)
        body.append(fileData)
        body.append(lineBreak.data(using: .utf8)!)
        body.append("--\(boundary)--\(lineBreak)".data(using: .utf8)!)
        return body
    }

    private func mimeType(for fileURL: URL) -> String {
        if let type = UTType(filenameExtension: fileURL.pathExtension),
           let preferred = type.preferredMIMEType {
            return preferred
        }
        return "application/octet-stream"
    }
}

private struct EmptyResponse: Decodable {}

private struct NoteCreateResponse: Decodable {
    let id: String
}

struct AudioUploadResponse: Decodable {
    let filename: String
    let url: String
    let contentType: String
}

private struct DocumentUploadResponse: Decodable {
    let id: String
}
