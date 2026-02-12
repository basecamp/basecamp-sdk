// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListPeopleOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct ListPingablePeopleOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct ListForProjectPeopleOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class PeopleService: BaseService, @unchecked Sendable {
    public func me() async throws -> Person {
        return try await request(
            OperationInfo(service: "People", operation: "GetMyProfile", resourceType: "my_profile", isMutation: false),
            method: "GET",
            path: "/my/profile.json",
            retryConfig: Metadata.retryConfig(for: "GetMyProfile")
        )
    }

    public func get(personId: Int) async throws -> Person {
        return try await request(
            OperationInfo(service: "People", operation: "GetPerson", resourceType: "person", isMutation: false, resourceId: personId),
            method: "GET",
            path: "/people/\(personId)",
            retryConfig: Metadata.retryConfig(for: "GetPerson")
        )
    }

    public func listAssignable() async throws -> [Person] {
        return try await request(
            OperationInfo(service: "People", operation: "ListAssignablePeople", resourceType: "assignable_people", isMutation: false),
            method: "GET",
            path: "/reports/todos/assigned.json",
            retryConfig: Metadata.retryConfig(for: "ListAssignablePeople")
        )
    }

    public func list(options: ListPeopleOptions? = nil) async throws -> ListResult<Person> {
        return try await requestPaginated(
            OperationInfo(service: "People", operation: "ListPeople", resourceType: "people", isMutation: false),
            path: "/people.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListPeople")
        )
    }

    public func listPingable(options: ListPingablePeopleOptions? = nil) async throws -> ListResult<Person> {
        return try await requestPaginated(
            OperationInfo(service: "People", operation: "ListPingablePeople", resourceType: "pingable_people", isMutation: false),
            path: "/circles/people.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListPingablePeople")
        )
    }

    public func listForProject(projectId: Int, options: ListForProjectPeopleOptions? = nil) async throws -> ListResult<Person> {
        return try await requestPaginated(
            OperationInfo(service: "People", operation: "ListProjectPeople", resourceType: "project_people", isMutation: false, projectId: projectId),
            path: "/projects/\(projectId)/people.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListProjectPeople")
        )
    }

    public func updateProjectAccess(projectId: Int, req: UpdateProjectAccessRequest) async throws -> ProjectAccessResult {
        return try await request(
            OperationInfo(service: "People", operation: "UpdateProjectAccess", resourceType: "project_access", isMutation: true, projectId: projectId),
            method: "PUT",
            path: "/projects/\(projectId)/people/users.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateProjectAccess")
        )
    }
}
