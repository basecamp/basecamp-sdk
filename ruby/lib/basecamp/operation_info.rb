# frozen_string_literal: true

module Basecamp
  # Information about a service operation for observability hooks.
  OperationInfo = Data.define(:service, :operation, :resource_type, :is_mutation, :project_id, :resource_id) do
    def initialize(service:, operation:, resource_type: nil, is_mutation: false, project_id: nil, resource_id: nil)
      super
    end
  end
end
