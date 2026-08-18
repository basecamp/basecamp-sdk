// A stand-in for a customer's app. This target imports the SDK the way an
// external package does — plain `import Basecamp`, never `@testable import` —
// and it exists solely to be compiled.
//
// Why it exists (#735): Swift's implicit memberwise initializer is `internal`,
// so a generated model with no explicit `public init` is unconstructible
// outside the module. Every test source in Tests/BasecampTests that imports the
// SDK imports it as `@testable import Basecamp`, which raises internal to
// visible, and none plain-imports it — so 35 all-optional models, two of them
// *request* payloads whose operations were therefore uncallable, compiled fine
// in-repo and shipped broken. Nothing in the package built against the public
// surface, so no CI job modelled a customer. The missing init was the defect;
// the missing consumer is why it took a customer to find it.
//
// Two rules keep this target honest, both enforced by PublicInitCoverageTests:
//
//   1. It must never use `@testable`. That single word re-opens the exact
//      blind spot this target closes, and it is the natural "fix" for anyone
//      hitting a compile error here.
//   2. It must stay a non-test target. `swift build` compiles it — so
//      `make swift-build`, `make swift-check`, the Swift CI job, the release
//      workflow, and the CodeQL Swift build all cover it, not just `swift test`.
//
// One way this target is *not* a customer, stated so nobody assumes otherwise:
// it lives in the same SwiftPM package, so `package`-level declarations — e.g.
// `BasecampClient.httpClient` — are visible here and are not visible to an
// external package. Nothing below touches one, and nothing added below should.
// Closing that last gap would mean a separate nested package, which would drop
// out of `swift build` and need its own CI step; the failure class this target
// exists for is internal-vs-public, which it does observe.
//
// Nothing here performs I/O; the async entry points are type-checked, never run.

import Basecamp
import Foundation

/// Public-surface exercises. Every member is written the way a consumer would
/// write it, using only what `import Basecamp` exports.
public enum PublicAPIConsumer {
    // MARK: - The #735 case: all-optional request payloads

    /// `GaugeNeedleUpdatePayload` has one optional member and no required one.
    /// Before #735 this function could not be written outside the module: the
    /// payload had no `public init`, so `UpdateGaugeNeedleRequest(gaugeNeedle:)`
    /// could only ever be handed `nil` — an empty `{}` PUT that bc3 rejects.
    public static func updateGaugeNeedleDescription(
        account: AccountClient,
        needleId: Int,
        description: String
    ) async throws -> GaugeNeedle {
        var payload = GaugeNeedleUpdatePayload()
        payload.description = description
        return try await account.gauges.updateGaugeNeedle(
            needleId: needleId,
            req: UpdateGaugeNeedleRequest(gaugeNeedle: payload)
        )
    }

    /// The same defect on the other affected operation: `PreferencesPayload` is
    /// all-optional, and `UpdateMyPreferences` carries nothing else.
    public static func updateMyTimeZone(
        account: AccountClient,
        timeZoneName: String
    ) async throws -> Preferences {
        let payload = PreferencesPayload(timeZoneName: timeZoneName)
        return try await account.people.updateMyPreferences(
            req: UpdateMyPreferencesRequest(person: payload)
        )
    }

    // MARK: - Client construction

    /// The documented entry point, exercised end to end so the consumer path is
    /// not just model construction.
    public static func makeAccountClient(accessToken: String, accountId: String) -> AccountClient {
        let client = BasecampClient(
            accessToken: accessToken,
            userAgent: "PublicAPIConsumer/1.0 (sdk@basecamp.com)"
        )
        return client.forAccount(accountId)
    }

    // MARK: - Every all-optional model, constructed from outside the module

    /// The full roster of generated models that carry no required member, each
    /// built with the zero-argument initializer a consumer needs.
    ///
    /// Listing all of them rather than a sample is deliberate: each line is an
    /// independent compile-time assertion, so reverting the generator fix
    /// produces 35 errors here instead of one that could be argued away. A
    /// model that disappears or is renamed also breaks this list, which is the
    /// intended signal — the roster is meant to be re-read, not auto-followed.
    ///
    /// The complementary guarantee lives in `PublicInitCoverageTests`, which scans
    /// *every* generated model for a `public init` and so covers models added
    /// after this list was written.
    public static func allOptionalModelsAreConstructibleFromOutsideTheModule() {
        _ = AccountLimits()
        _ = AccountLogo()
        _ = AccountSettings()
        _ = AccountSubscription()
        _ = CampfireLineAttachment()
        _ = ClientApprovalResponse()
        _ = ClientSide()
        _ = CreateAttachmentResponseContent()
        _ = DoorService()
        _ = EventDetails()
        _ = EverythingFile()
        _ = GaugeNeedleUpdatePayload()
        _ = GetAssignedTodosResponseContent()
        _ = GetMyAssignmentsResponseContent()
        _ = GetOverdueTodosResponseContent()
        // GetPersonProgressResponseContent left this list when its two members
        // were modeled @required (#728): it has no zero-parameter init any more.
        // Its public init is still covered — by the reflective sweep in
        // PublicInitCoverageTests, which reads every generated model.
        _ = GetPersonProgressResponseContent(
            events: [], person: Person(id: 1, name: "Victor Cooper"))
        _ = OutOfOffice()
        _ = PauseQuestionResponseContent()
        _ = Preferences()
        _ = PreferencesPayload()
        _ = PreviewableAttachment()
        _ = ProjectAccessResult()
        _ = QuestionReminder()
        _ = QuestionSchedule()
        _ = ResumeQuestionResponseContent()
        _ = ScheduleAttributes()
        _ = TimelineAttachment()
        _ = TimelineEvent()
        _ = UpdateQuestionNotificationSettingsResponseContent()
        _ = WebhookCopy()
        _ = WebhookCopyBucket()
        _ = WebhookDelivery()
        _ = WebhookDeliveryRequest()
        _ = WebhookDeliveryResponse()
        _ = WebhookEvent()
    }

    // MARK: - Models that already had an init, so the fix is checked both ways

    /// `GaugeNeedlePayload` has a required `position` and was always
    /// constructible. Keeping it here proves the generator change widened the
    /// init to all-optional models without altering the required-member shape:
    /// required parameters still take no default, optional ones still default
    /// to nil.
    public static func requiredMemberModelsKeepTheirInitShape() {
        _ = GaugeNeedlePayload(position: 50)
        _ = GaugeNeedlePayload(position: 50, color: "#00ff00", description: "<div>note</div>")
    }
}
