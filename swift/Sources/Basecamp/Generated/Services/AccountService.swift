// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class AccountService: BaseService, @unchecked Sendable {
    public func account() async throws -> Account {
        return try await request(
            OperationInfo(service: "Account", operation: "GetAccount", resourceType: "account", isMutation: false),
            method: "GET",
            path: "/account.json",
            retryConfig: Metadata.retryConfig(for: "GetAccount")
        )
    }

    public func removeAccountLogo() async throws {
        try await requestVoid(
            OperationInfo(service: "Account", operation: "RemoveAccountLogo", resourceType: "resource", isMutation: true),
            method: "DELETE",
            path: "/account/logo.json",
            retryConfig: Metadata.retryConfig(for: "RemoveAccountLogo")
        )
    }

    public func updateAccountLogo(data: Data, filename: String, contentType: String) async throws {
        let boundary = UUID().uuidString
        let multipartContentType = "multipart/form-data; boundary=\(boundary)"
        // Sanitize user-provided values to prevent CRLF injection / quote breakout
        let safeFilename = filename.replacingOccurrences(of: "\r", with: "").replacingOccurrences(of: "\n", with: "").replacingOccurrences(of: "\\", with: "\\\\").replacingOccurrences(of: "\"", with: "\\\"")
        let safeContentType = contentType.replacingOccurrences(of: "\r", with: "").replacingOccurrences(of: "\n", with: "")
        var body = Data()
        body.append(contentsOf: "--\(boundary)\r\n".utf8)
        body.append(contentsOf: "Content-Disposition: form-data; name=\"logo\"; filename=\"\(safeFilename)\"\r\n".utf8)
        body.append(contentsOf: "Content-Type: \(safeContentType)\r\n\r\n".utf8)
        body.append(data)
        body.append(contentsOf: "\r\n--\(boundary)--\r\n".utf8)
        let multipartBody = body
        try await requestVoid(
            OperationInfo(service: "Account", operation: "UpdateAccountLogo", resourceType: "account_logo", isMutation: true),
            method: "PUT",
            path: "/account/logo.json",
            body: multipartBody,
            contentType: multipartContentType,
            retryConfig: Metadata.retryConfig(for: "UpdateAccountLogo")
        )
    }

    public func updateAccountName(req: UpdateAccountNameRequest) async throws -> Account {
        return try await request(
            OperationInfo(service: "Account", operation: "UpdateAccountName", resourceType: "account_name", isMutation: true),
            method: "PUT",
            path: "/account/name.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateAccountName")
        )
    }
}
