// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct SearchMetadata: Codable, Sendable {
    public let defaultBucketLabel: String
    public let defaultCircleLabel: String
    public let defaultCreatorLabel: String
    public let defaultFileTypeLabel: String
    public let defaultTypeLabel: String
    public let fileSearchTypes: [SearchType]
    public let recordingSearchTypes: [SearchType]

    public init(
        defaultBucketLabel: String,
        defaultCircleLabel: String,
        defaultCreatorLabel: String,
        defaultFileTypeLabel: String,
        defaultTypeLabel: String,
        fileSearchTypes: [SearchType],
        recordingSearchTypes: [SearchType]
    ) {
        self.defaultBucketLabel = defaultBucketLabel
        self.defaultCircleLabel = defaultCircleLabel
        self.defaultCreatorLabel = defaultCreatorLabel
        self.defaultFileTypeLabel = defaultFileTypeLabel
        self.defaultTypeLabel = defaultTypeLabel
        self.fileSearchTypes = fileSearchTypes
        self.recordingSearchTypes = recordingSearchTypes
    }
}
