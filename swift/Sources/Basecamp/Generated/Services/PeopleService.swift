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
    public func disableOutOfOffice(personId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "People", operation: "DisableOutOfOffice", resourceType: "out_of_office", isMutation: true, resourceId: personId),
            method: "DELETE",
            path: "/people/\(personId)/out_of_office.json",
            retryConfig: Metadata.retryConfig(for: "DisableOutOfOffice")
        )
    }

    public func enableOutOfOffice(personId: Int, req: EnableOutOfOfficeRequest) async throws -> OutOfOffice {
        return try await request(
            OperationInfo(service: "People", operation: "EnableOutOfOffice", resourceType: "out_of_office", isMutation: true, resourceId: personId),
            method: "POST",
            path: "/people/\(personId)/out_of_office.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "EnableOutOfOffice")
        )
    }

    public func myPreferences() async throws -> Preferences {
        return try await request(
            OperationInfo(service: "People", operation: "GetMyPreferences", resourceType: "my_preference", isMutation: false),
            method: "GET",
            path: "/my/preferences.json",
            retryConfig: Metadata.retryConfig(for: "GetMyPreferences")
        )
    }

    public func me() async throws -> Person {
        return try await request(
            OperationInfo(service: "People", operation: "GetMyProfile", resourceType: "my_profile", isMutation: false),
            method: "GET",
            path: "/my/profile.json",
            retryConfig: Metadata.retryConfig(for: "GetMyProfile")
        )
    }

    public func outOfOffice(personId: Int) async throws -> OutOfOffice {
        return try await request(
            OperationInfo(service: "People", operation: "GetOutOfOffice", resourceType: "out_of_office", isMutation: false, resourceId: personId),
            method: "GET",
            path: "/people/\(personId)/out_of_office.json",
            retryConfig: Metadata.retryConfig(for: "GetOutOfOffice")
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

    public func updateMyPreferences(req: UpdateMyPreferencesRequest) async throws -> Preferences {
        return try await request(
            OperationInfo(service: "People", operation: "UpdateMyPreferences", resourceType: "my_preference", isMutation: true),
            method: "PUT",
            path: "/my/preferences.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateMyPreferences")
        )
    }

    public func updateMyProfile(req: UpdateMyProfileRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "People", operation: "UpdateMyProfile", resourceType: "my_profile", isMutation: true),
            method: "PUT",
            path: "/my/profile.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateMyProfile")
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
