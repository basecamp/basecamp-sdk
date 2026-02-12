// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class CardStepsService: BaseService, @unchecked Sendable {
    public func create(projectId: Int, cardId: Int, req: CreateCardStepRequest) async throws -> CardStep {
        return try await request(
            OperationInfo(service: "CardSteps", operation: "CreateCardStep", resourceType: "card_step", isMutation: true, projectId: projectId, resourceId: cardId),
            method: "POST",
            path: "/buckets/\(projectId)/card_tables/cards/\(cardId)/steps.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateCardStep")
        )
    }

    public func reposition(projectId: Int, cardId: Int, req: RepositionCardStepRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "CardSteps", operation: "RepositionCardStep", resourceType: "card_step", isMutation: true, projectId: projectId, resourceId: cardId),
            method: "POST",
            path: "/buckets/\(projectId)/card_tables/cards/\(cardId)/positions.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "RepositionCardStep")
        )
    }

    public func setCompletion(projectId: Int, stepId: Int, req: SetCardStepCompletionRequest) async throws -> CardStep {
        return try await request(
            OperationInfo(service: "CardSteps", operation: "SetCardStepCompletion", resourceType: "card_step_completion", isMutation: true, projectId: projectId, resourceId: stepId),
            method: "PUT",
            path: "/buckets/\(projectId)/card_tables/steps/\(stepId)/completions.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "SetCardStepCompletion")
        )
    }

    public func update(projectId: Int, stepId: Int, req: UpdateCardStepRequest) async throws -> CardStep {
        return try await request(
            OperationInfo(service: "CardSteps", operation: "UpdateCardStep", resourceType: "card_step", isMutation: true, projectId: projectId, resourceId: stepId),
            method: "PUT",
            path: "/buckets/\(projectId)/card_tables/steps/\(stepId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateCardStep")
        )
    }
}
