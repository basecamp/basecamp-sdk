# frozen_string_literal: true

require "test_helper"

# Accessor-roster inventory: every generated service must be *reachable*.
#
# check-ruby-service-drift.sh proves the generated service directory matches the
# spec, and `make check-service-inventory-parity` proves the five generators
# emitted the same service set. Neither asks whether AccountClient exposes what
# was generated — the accessors in client.rb's `# @!group Services` block are
# hand-written, and a missing one breaks nothing: nothing references it, so the
# suite stays green and the service is simply unreachable. Python shipped
# `gauges` that way for about a year (#732, #755).
#
# The roster is derived from the generated directory rather than typed out here,
# so a newly generated service enters this test the moment it is generated and
# fails until someone wires an accessor. A hand-copied literal would have to be
# edited by the same person who forgot the accessor.
#
# Ruby needs no subclass allowance, unlike Python and TypeScript: the five
# hand-written composites are `prepend`ed onto the generated classes by the
# zeitwerk `on_load` hooks in lib/basecamp.rb, so the generated constant is
# still the accessor's exact class.
class AccessorInventoryTest < Minitest::Test
  include TestHelper

  GENERATED_SERVICES_DIR = File.expand_path("../../lib/basecamp/generated/services", __dir__)

  # Hand-written infrastructure living beside the generated services.
  NON_GENERATED_SERVICE_FILES = [ "base_service.rb" ].freeze

  # Zero-arity public methods on AccountClient that are not service accessors.
  INFRASTRUCTURE_METHODS = %i[account_id config http hooks].freeze

  # Non-vacuity floor. Every assertion below iterates a derived roster, so a
  # path change that yielded an empty roster would make all of them vacuously
  # true. Kept well under the real count (53) so it is not a second constant to
  # maintain.
  MIN_GENERATED_SERVICES = 40

  # accessor name (snake_case) => generated class constant name.
  ROSTER = Dir.children(GENERATED_SERVICES_DIR)
    .select { |f| f.end_with?("_service.rb") }
    .reject { |f| NON_GENERATED_SERVICE_FILES.include?(f) }
    .to_h do |file|
      accessor = File.basename(file, "_service.rb")
      [ accessor.to_sym, "#{accessor.split("_").map(&:capitalize).join}Service" ]
    end
    .freeze

  def test_roster_is_derived_and_not_empty
    assert_operator ROSTER.size, :>, MIN_GENERATED_SERVICES,
      "roster derivation yielded #{ROSTER.size} services; every assertion in this file would be vacuous"
  end

  def test_every_generated_service_has_an_accessor
    account = create_client.for_account(account_id)

    ROSTER.each_key do |accessor|
      assert_respond_to account, accessor,
        "AccountClient has no `#{accessor}` accessor; generated/services/#{accessor}_service.rb " \
        "defines that service but client.rb does not wire it"
    end
  end

  def test_each_accessor_resolves_to_that_service
    account = create_client.for_account(account_id)

    ROSTER.each do |accessor, class_name|
      klass = Basecamp::Services.const_get(class_name)

      # Presence alone would pass if `gauges` returned the wrong service, so
      # resolve it and check the class it actually yields.
      assert_instance_of klass, account.public_send(accessor),
        "AccountClient##{accessor} does not return #{class_name}"
    end
  end

  def test_generated_classes_are_all_constants
    # Guards the class-name derivation itself: one rule turns a filename into a
    # constant name, and an irregular spelling should fail here by name rather
    # than as a NameError buried in the identity test above.
    ROSTER.each do |accessor, class_name|
      assert Basecamp::Services.const_defined?(class_name),
        "#{accessor}_service.rb does not define Basecamp::Services::#{class_name}"
    end
  end

  def test_no_accessor_without_a_generated_service
    # The other direction: an accessor left behind by a removed or renamed
    # service. Every service accessor is a zero-arity public method defined
    # directly on AccountClient; the four infrastructure readers are the only
    # others, and they are named rather than derived so a new one has to be
    # acknowledged here.
    #
    # The class is read off an instance rather than named as a constant.
    # `Basecamp::AccountClient` is declared inside client.rb alongside
    # `Basecamp::Client`, so zeitwerk has no autoload registered for it: naming
    # it directly resolves only once something else has already loaded that
    # file, which under minitest's random seed is a coin flip (it raised
    # NameError on the first seed that ran this test first).
    account_client_class = create_client.for_account(account_id).class

    zero_arity = account_client_class.public_instance_methods(false)
      .select { |m| account_client_class.instance_method(m).arity.zero? }

    assert_equal ROSTER.keys.sort, (zero_arity - INFRASTRUCTURE_METHODS).sort
  end

  def test_authorization_is_not_an_account_scoped_service
    # Stated so the reverse assertion above cannot be quietly loosened to
    # accommodate it: `authorization` is an OAuth service on the outer Client,
    # outside the account-scoped roster entirely. Asserted against the same
    # method list that assertion enumerates, not against `respond_to?`, so the
    # two cannot disagree about where the accessor lives.
    client = create_client

    assert_respond_to client, :authorization
    assert_not_includes client.for_account(account_id).class.public_instance_methods(false), :authorization
  end
end
