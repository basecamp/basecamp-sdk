// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class CardStepsService: BaseService, @unchecked Sendable {
    public func create(cardId: Int, req: CreateCardStepRequest) async throws -> CardStep {
        return try await request(
            OperationInfo(service: "CardSteps", operation: "CreateCardStep", resourceType: "card_step", isMutation: true, resourceId: cardId),
            method: "POST",
            path: "/card_tables/cards/\(cardId)/steps.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateCardStep")
        )
    }

    public func reposition(cardId: Int, req: RepositionCardStepRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "CardSteps", operation: "RepositionCardStep", resourceType: "card_step", isMutation: true, resourceId: cardId),
            method: "POST",
            path: "/card_tables/cards/\(cardId)/positions.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "RepositionCardStep")
        )
    }

    public func setCompletion(stepId: Int, req: SetCardStepCompletionRequest) async throws -> CardStep {
        return try await request(
            OperationInfo(service: "CardSteps", operation: "SetCardStepCompletion", resourceType: "card_step_completion", isMutation: true, resourceId: stepId),
            method: "PUT",
            path: "/card_tables/steps/\(stepId)/completions.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "SetCardStepCompletion")
        )
    }

    public func update(stepId: Int, req: UpdateCardStepRequest) async throws -> CardStep {
        return try await request(
            OperationInfo(service: "CardSteps", operation: "UpdateCardStep", resourceType: "card_step", isMutation: true, resourceId: stepId),
            method: "PUT",
            path: "/card_tables/steps/\(stepId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateCardStep")
        )
    }
}
