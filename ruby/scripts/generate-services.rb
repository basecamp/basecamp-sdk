#!/usr/bin/env ruby
# frozen_string_literal: true

# Generates Ruby service classes from OpenAPI spec.
#
# Usage: ruby scripts/generate-services.rb [--openapi ../openapi.json] [--output lib/basecamp/generated/services]
#
# This generator:
# 1. Parses openapi.json
# 2. Groups operations by tag
# 3. Maps operationIds to method names
# 4. Generates Ruby service files

require 'json'
require 'fileutils'

# Service generator for Ruby SDK
class ServiceGenerator
  METHODS = %w[get post put patch delete].freeze

  # Schema reference cache for resolving $ref
  attr_reader :schemas

  # Tag to service name mapping overrides
  TAG_TO_SERVICE = {
    'Card Tables' => 'CardTables',
    'Campfire' => 'Campfires',
    'Todos' => 'Todos',
    'Messages' => 'Messages',
    'Files' => 'Files',
    'Forwards' => 'Forwards',
    'Schedule' => 'Schedules',
    'People' => 'People',
    'Projects' => 'Projects',
    'Automation' => 'Automation',
    'ClientFeatures' => 'ClientFeatures',
    'Boosts' => 'Boosts',
    'Untagged' => 'Miscellaneous'
  }.freeze

  # Service splits - some tags map to multiple services
  SERVICE_SPLITS = {
    'Campfire' => {
      'Campfires' => %w[
        GetCampfire ListCampfires
        ListChatbots CreateChatbot GetChatbot UpdateChatbot DeleteChatbot
        ListCampfireLines CreateCampfireLine GetCampfireLine DeleteCampfireLine
      ]
    },
    'Card Tables' => {
      'CardTables' => %w[GetCardTable],
      'Cards' => %w[GetCard UpdateCard MoveCard CreateCard ListCards],
      'CardColumns' => %w[
        GetCardColumn UpdateCardColumn SetCardColumnColor
        EnableCardColumnOnHold DisableCardColumnOnHold
        CreateCardColumn MoveCardColumn
      ],
      'CardSteps' => %w[
        CreateCardStep UpdateCardStep SetCardStepCompletion
        RepositionCardStep
      ]
    },
    'Files' => {
      'Attachments' => %w[CreateAttachment],
      'Uploads' => %w[GetUpload UpdateUpload ListUploads CreateUpload ListUploadVersions],
      'Vaults' => %w[GetVault UpdateVault ListVaults CreateVault],
      'Documents' => %w[GetDocument UpdateDocument ListDocuments CreateDocument]
    },
    'Automation' => {
      'Tools' => %w[GetTool UpdateTool DeleteTool CloneTool EnableTool DisableTool RepositionTool],
      'Recordings' => %w[GetRecording ArchiveRecording UnarchiveRecording TrashRecording ListRecordings],
      'Webhooks' => %w[ListWebhooks CreateWebhook GetWebhook UpdateWebhook DeleteWebhook],
      'Events' => %w[ListEvents],
      'Lineup' => %w[CreateLineupMarker UpdateLineupMarker DeleteLineupMarker],
      'Search' => %w[Search GetSearchMetadata],
      'Templates' => %w[
        ListTemplates CreateTemplate GetTemplate UpdateTemplate
        DeleteTemplate CreateProjectFromTemplate GetProjectConstruction
      ],
      'Checkins' => %w[
        GetQuestionnaire ListQuestions CreateQuestion GetQuestion
        UpdateQuestion ListAnswers CreateAnswer GetAnswer UpdateAnswer
      ]
    },
    'Messages' => {
      'Messages' => %w[GetMessage UpdateMessage CreateMessage ListMessages PinMessage UnpinMessage],
      'MessageBoards' => %w[GetMessageBoard],
      'MessageTypes' => %w[
        ListMessageTypes CreateMessageType GetMessageType
        UpdateMessageType DeleteMessageType
      ],
      'Comments' => %w[GetComment UpdateComment ListComments CreateComment]
    },
    'People' => {
      'People' => %w[
        GetMyProfile ListPeople GetPerson ListProjectPeople
        UpdateProjectAccess ListPingablePeople
      ],
      'Subscriptions' => %w[GetSubscription Subscribe Unsubscribe UpdateSubscription]
    },
    'Schedule' => {
      'Schedules' => %w[
        GetSchedule UpdateScheduleSettings ListScheduleEntries
        CreateScheduleEntry GetScheduleEntry UpdateScheduleEntry
        GetScheduleEntryOccurrence
      ],
      'Timesheets' => %w[GetRecordingTimesheet GetProjectTimesheet GetTimesheetReport GetTimesheetEntry CreateTimesheetEntry UpdateTimesheetEntry]
    },
    'ClientFeatures' => {
      'ClientApprovals' => %w[ListClientApprovals GetClientApproval],
      'ClientCorrespondences' => %w[ListClientCorrespondences GetClientCorrespondence],
      'ClientReplies' => %w[ListClientReplies GetClientReply],
      'ClientVisibility' => %w[SetClientVisibility]
    },
    'Todos' => {
      'Todos' => %w[ListTodos CreateTodo GetTodo UpdateTodo CompleteTodo UncompleteTodo TrashTodo],
      'Todolists' => %w[GetTodolistOrGroup UpdateTodolistOrGroup ListTodolists CreateTodolist],
      'Todosets' => %w[GetTodoset],
      'TodolistGroups' => %w[ListTodolistGroups CreateTodolistGroup RepositionTodolistGroup]
    },
    'Untagged' => {
      'Timeline' => %w[GetProjectTimeline],
      'Reports' => %w[GetProgressReport GetUpcomingSchedule GetAssignedTodos GetOverdueTodos GetPersonProgress],
      'Checkins' => %w[
        GetQuestionReminders ListQuestionAnswerers GetAnswersByPerson
        UpdateQuestionNotificationSettings PauseQuestion ResumeQuestion
      ],
      'Todos' => %w[RepositionTodo],
      'People' => %w[ListAssignablePeople],
      'CardColumns' => %w[SubscribeToCardColumn UnsubscribeFromCardColumn]
    }
  }.freeze

  # Method name overrides
  METHOD_NAME_OVERRIDES = {
    'GetMyProfile' => 'my_profile',
    'GetTodolistOrGroup' => 'get',
    'UpdateTodolistOrGroup' => 'update',
    'SetCardColumnColor' => 'set_color',
    'EnableCardColumnOnHold' => 'enable_on_hold',
    'DisableCardColumnOnHold' => 'disable_on_hold',
    'RepositionCardStep' => 'reposition',
    'CreateCardStep' => 'create',
    'UpdateCardStep' => 'update',
    'SetCardStepCompletion' => 'set_completion',
    'GetQuestionnaire' => 'get_questionnaire',
    'GetQuestion' => 'get_question',
    'GetAnswer' => 'get_answer',
    'ListQuestions' => 'list_questions',
    'ListAnswers' => 'list_answers',
    'CreateQuestion' => 'create_question',
    'CreateAnswer' => 'create_answer',
    'UpdateQuestion' => 'update_question',
    'UpdateAnswer' => 'update_answer',
    'GetQuestionReminders' => 'reminders',
    'GetAnswersByPerson' => 'by_person',
    'ListQuestionAnswerers' => 'answerers',
    'UpdateQuestionNotificationSettings' => 'update_notification_settings',
    'PauseQuestion' => 'pause',
    'ResumeQuestion' => 'resume',
    'GetSearchMetadata' => 'metadata',
    'Search' => 'search',
    'CreateProjectFromTemplate' => 'create_project',
    'GetProjectConstruction' => 'get_construction',
    'GetRecordingTimesheet' => 'for_recording',
    'GetProjectTimesheet' => 'for_project',
    'GetTimesheetReport' => 'report',
    'GetTimesheetEntry' => 'get',
    'CreateTimesheetEntry' => 'create',
    'UpdateTimesheetEntry' => 'update',
    'GetProgressReport' => 'progress',
    'GetUpcomingSchedule' => 'upcoming',
    'GetAssignedTodos' => 'assigned',
    'GetOverdueTodos' => 'overdue',
    'GetPersonProgress' => 'person_progress',
    'SubscribeToCardColumn' => 'subscribe_to_column',
    'UnsubscribeFromCardColumn' => 'unsubscribe_from_column',
    'SetClientVisibility' => 'set_visibility',
    # Campfires - use specific names to avoid conflicts between campfire, chatbots, and lines
    'GetCampfire' => 'get',
    'ListCampfires' => 'list',
    'ListChatbots' => 'list_chatbots',
    'CreateChatbot' => 'create_chatbot',
    'GetChatbot' => 'get_chatbot',
    'UpdateChatbot' => 'update_chatbot',
    'DeleteChatbot' => 'delete_chatbot',
    'ListCampfireLines' => 'list_lines',
    'CreateCampfireLine' => 'create_line',
    'GetCampfireLine' => 'get_line',
    'DeleteCampfireLine' => 'delete_line',
    # Forwards - use specific names to avoid conflicts between forwards, replies, and inbox
    'GetForward' => 'get',
    'ListForwards' => 'list',
    'GetForwardReply' => 'get_reply',
    'ListForwardReplies' => 'list_replies',
    'CreateForwardReply' => 'create_reply',
    'GetInbox' => 'get_inbox',
    # Uploads - use specific names to avoid conflicts with versions
    'GetUpload' => 'get',
    'UpdateUpload' => 'update',
    'ListUploads' => 'list',
    'CreateUpload' => 'create',
    'ListUploadVersions' => 'list_versions',
    'GetMessage' => 'get',
    'UpdateMessage' => 'update',
    'CreateMessage' => 'create',
    'ListMessages' => 'list',
    'PinMessage' => 'pin',
    'UnpinMessage' => 'unpin',
    'GetMessageBoard' => 'get',
    'GetMessageType' => 'get',
    'UpdateMessageType' => 'update',
    'CreateMessageType' => 'create',
    'ListMessageTypes' => 'list',
    'DeleteMessageType' => 'delete',
    'GetComment' => 'get',
    'UpdateComment' => 'update',
    'CreateComment' => 'create',
    'ListComments' => 'list',
    'ListProjectPeople' => 'list_for_project',
    'ListPingablePeople' => 'list_pingable',
    'ListAssignablePeople' => 'list_assignable',
    'GetSchedule' => 'get',
    'UpdateScheduleSettings' => 'update_settings',
    'GetScheduleEntry' => 'get_entry',
    'UpdateScheduleEntry' => 'update_entry',
    'CreateScheduleEntry' => 'create_entry',
    'ListScheduleEntries' => 'list_entries',
    'GetScheduleEntryOccurrence' => 'get_entry_occurrence'
  }.freeze

  # Verb patterns for extracting method names
  VERB_PATTERNS = [
    { prefix: 'Subscribe', method: 'subscribe' },
    { prefix: 'Unsubscribe', method: 'unsubscribe' },
    { prefix: 'List', method: 'list' },
    { prefix: 'Get', method: 'get' },
    { prefix: 'Create', method: 'create' },
    { prefix: 'Update', method: 'update' },
    { prefix: 'Delete', method: 'delete' },
    { prefix: 'Trash', method: 'trash' },
    { prefix: 'Archive', method: 'archive' },
    { prefix: 'Unarchive', method: 'unarchive' },
    { prefix: 'Complete', method: 'complete' },
    { prefix: 'Uncomplete', method: 'uncomplete' },
    { prefix: 'Enable', method: 'enable' },
    { prefix: 'Disable', method: 'disable' },
    { prefix: 'Reposition', method: 'reposition' },
    { prefix: 'Move', method: 'move' },
    { prefix: 'Clone', method: 'clone' },
    { prefix: 'Set', method: 'set' },
    { prefix: 'Pin', method: 'pin' },
    { prefix: 'Unpin', method: 'unpin' },
    { prefix: 'Pause', method: 'pause' },
    { prefix: 'Resume', method: 'resume' },
    { prefix: 'Search', method: 'search' }
  ].freeze

  SIMPLE_RESOURCES = %w[
    todo todos todolist todolists todoset message messages comment comments
    card cards cardtable cardcolumn cardstep column step project projects
    person people campfire campfires chatbot chatbots webhook webhooks
    vault vaults document documents upload uploads schedule scheduleentry
    scheduleentries event events recording recordings template templates
    attachment question questions answer answers questionnaire subscription
    forward forwards inbox messageboard messagetype messagetypes tool
    lineupmarker clientapproval clientapprovals clientcorrespondence
    clientcorrespondences clientreply clientreplies forwardreply
    forwardreplies campfireline campfirelines todolistgroup todolistgroups
    todolistorgroup uploadversions
  ].freeze

  def initialize(openapi_path)
    @openapi = JSON.parse(File.read(openapi_path))
    @schemas = @openapi.dig('components', 'schemas') || {}
  end

  def generate(output_dir)
    FileUtils.mkdir_p(output_dir)

    services = group_operations
    generated_files = []

    services.each do |name, service|
      code = generate_service(service)
      filename = "#{to_snake_case(name)}_service.rb"
      filepath = File.join(output_dir, filename)
      File.write(filepath, code)
      generated_files << filename
      puts "Generated #{filename} (#{service[:operations].length} operations)"
    end

    puts "\nGenerated #{services.length} services with #{services.values.sum { |s| s[:operations].length }} operations total."
    generated_files
  end

  private

  def group_operations
    services = {}

    @openapi['paths'].each do |path, path_item|
      METHODS.each do |method|
        operation = path_item[method]
        next unless operation

        tag = operation['tags']&.first || 'Untagged'
        parsed = parse_operation(path, method, operation)

        # Determine which service this operation belongs to
        service_name = find_service_for_operation(tag, operation['operationId'])

        services[service_name] ||= {
          name: service_name,
          class_name: "#{service_name}Service",
          description: "Service for #{service_name} operations",
          operations: []
        }

        services[service_name][:operations] << parsed
      end
    end

    services
  end

  def find_service_for_operation(tag, operation_id)
    if SERVICE_SPLITS[tag]
      SERVICE_SPLITS[tag].each do |svc, op_ids|
        return svc if op_ids.include?(operation_id)
      end
    end

    TAG_TO_SERVICE[tag] || tag.gsub(/\s+/, '')
  end

  def parse_operation(path, method, operation)
    operation_id = operation['operationId']
    method_name = extract_method_name(operation_id)
    http_method = method.upcase
    description = operation['description']&.lines&.first&.strip || "#{method_name} operation"

    # Extract path parameters (excluding accountId)
    path_params = (operation['parameters'] || [])
                  .select { |p| p['in'] == 'path' && p['name'] != 'accountId' }
                  .map { |p| { name: p['name'], type: schema_to_ruby_type(p['schema']), description: p['description'] } }

    # Extract query parameters
    query_params = (operation['parameters'] || [])
                   .select { |p| p['in'] == 'query' }
                   .map do |p|
      {
        name: p['name'],
        type: schema_to_ruby_type(p['schema']),
        required: p['required'] || false,
        description: p['description']
      }
    end

    # Check for request body (JSON or binary)
    body_schema_ref = operation.dig('requestBody', 'content', 'application/json', 'schema')
    has_binary_body = operation.dig('requestBody', 'content', 'application/octet-stream', 'schema')

    # Extract body parameters from schema
    body_params = extract_body_params(body_schema_ref)

    # Check response
    success_response = operation.dig('responses', '200') || operation.dig('responses', '201')
    response_schema = success_response&.dig('content', 'application/json', 'schema')
    returns_void = response_schema.nil?
    returns_array = response_schema&.dig('type') == 'array'

    {
      operation_id: operation_id,
      method_name: method_name,
      http_method: http_method,
      path: convert_path(path),
      description: description,
      path_params: path_params,
      query_params: query_params,
      body_params: body_params,
      has_body: body_params.any?,
      has_binary_body: !!has_binary_body,
      returns_void: returns_void,
      returns_array: returns_array,
      is_mutation: http_method != 'GET',
      has_pagination: !!operation['x-basecamp-pagination']
    }
  end

  # Extract body parameters from a schema reference
  def extract_body_params(schema_ref)
    return [] unless schema_ref

    # Resolve $ref
    schema = resolve_schema_ref(schema_ref)
    return [] unless schema && schema['properties']

    required_fields = schema['required'] || []

    schema['properties'].map do |name, prop|
      type = schema_to_ruby_type(prop)
      format_hint = extract_format_hint(prop)
      {
        name: name,
        type: type,
        required: required_fields.include?(name),
        description: prop['description'],
        format_hint: format_hint
      }
    end
  end

  # Resolve a schema reference to its definition
  def resolve_schema_ref(schema_or_ref)
    return schema_or_ref unless schema_or_ref['$ref']

    ref_path = schema_or_ref['$ref']
    # Handle #/components/schemas/SchemaName format
    if ref_path.start_with?('#/components/schemas/')
      schema_name = ref_path.split('/').last
      @schemas[schema_name]
    else
      nil
    end
  end

  # Extract format hint for documentation
  def extract_format_hint(prop)
    return nil unless prop

    # Check for x-go-type hints (dates)
    case prop['x-go-type']
    when 'types.Date'
      return 'YYYY-MM-DD'
    when 'types.DateTime', 'time.Time'
      return 'RFC3339 (e.g., 2024-12-15T09:00:00Z)'
    end

    # Check for format field
    case prop['format']
    when 'date'
      'YYYY-MM-DD'
    when 'date-time'
      'RFC3339 (e.g., 2024-12-15T09:00:00Z)'
    end
  end

  def extract_method_name(operation_id)
    return METHOD_NAME_OVERRIDES[operation_id] if METHOD_NAME_OVERRIDES.key?(operation_id)

    VERB_PATTERNS.each do |pattern|
      if operation_id.start_with?(pattern[:prefix])
        remainder = operation_id[pattern[:prefix].length..]
        return pattern[:method] if remainder.empty?

        resource = to_snake_case(remainder)
        return pattern[:method] if simple_resource?(resource)

        return "#{pattern[:method]}_#{resource}"
      end
    end

    to_snake_case(operation_id)
  end

  def simple_resource?(resource)
    SIMPLE_RESOURCES.include?(resource.downcase.gsub('_', ''))
  end

  def convert_path(path)
    # Remove /{accountId} prefix
    path = path.sub(%r{^/\{accountId\}}, '')
    # Convert {camelCaseParam} to #{snake_case_param}
    path.gsub(/\{(\w+)\}/) do |_match|
      param = ::Regexp.last_match(1)
      snake_param = to_snake_case(param)
      "\#{#{snake_param}}"
    end
  end

  def schema_to_ruby_type(schema)
    return 'Object' unless schema

    case schema['type']
    when 'integer' then 'Integer'
    when 'boolean' then 'Boolean'
    when 'array' then 'Array'
    else 'String'
    end
  end

  def to_snake_case(str)
    str.gsub(/([a-z\d])([A-Z])/, '\1_\2')
       .gsub(/([A-Z]+)([A-Z][a-z])/, '\1_\2')
       .downcase
  end

  def generate_service(service)
    lines = []

    # Check if any operation uses URI encoding (binary uploads with query params)
    needs_uri = service[:operations].any? { |op| op[:has_binary_body] && op[:query_params].any? }

    lines << '# frozen_string_literal: true'
    lines << ''
    lines << 'require "uri"' if needs_uri
    lines << '' if needs_uri
    lines << 'module Basecamp'
    lines << '  module Services'
    lines << "    # #{service[:description]}"
    lines << '    #'
    lines << '    # @generated from OpenAPI spec'
    lines << "    class #{service[:class_name]} < BaseService"

    service[:operations].each do |op|
      lines << ''
      lines.concat(generate_method(op))
    end

    lines << '    end'
    lines << '  end'
    lines << 'end'
    lines << ''

    lines.join("\n")
  end

  def generate_method(op)
    lines = []

    # Method signature
    params = build_params(op)

    # YARD documentation
    lines << "      # #{op[:description]}"

    # Add @param tags for path params
    op[:path_params].each do |p|
      ruby_name = to_snake_case(p[:name])
      type = p[:type] || 'Integer'
      desc = p[:description] || "#{ruby_name.gsub('_', ' ')} ID"
      lines << "      # @param #{ruby_name} [#{type}] #{desc}"
    end

    # Add @param tags for binary upload params
    if op[:has_binary_body]
      lines << '      # @param data [String] Binary file data to upload'
      lines << '      # @param content_type [String] MIME type of the file (e.g., "application/pdf", "image/png")'
    end

    # Add @param tags for body params
    if op[:body_params]&.any?
      op[:body_params].each do |b|
        ruby_name = to_snake_case(b[:name])
        type = b[:type] || 'Object'
        type = "#{type}, nil" unless b[:required]
        desc = b[:description] || ruby_name.gsub('_', ' ')
        format_hint = b[:format_hint] ? " (#{b[:format_hint]})" : ''
        lines << "      # @param #{ruby_name} [#{type}] #{desc}#{format_hint}"
      end
    end

    # Add @param tags for query params
    op[:query_params].each do |q|
      ruby_name = to_snake_case(q[:name])
      type = q[:type] || 'String'
      type = "#{type}, nil" unless q[:required]
      desc = q[:description] || ruby_name.gsub('_', ' ')
      lines << "      # @param #{ruby_name} [#{type}] #{desc}"
    end

    # Add @return tag
    if op[:returns_void]
      lines << '      # @return [void]'
    elsif op[:returns_array] || op[:has_pagination]
      lines << '      # @return [Enumerator<Hash>] paginated results'
    else
      lines << '      # @return [Hash] response data'
    end

    lines << "      def #{op[:method_name]}(#{params})"

    # Build the path
    path_expr = build_path_expression(op)

    # Generate the method body based on operation type
    if op[:returns_void]
      lines.concat(generate_void_method_body(op, path_expr))
    elsif op[:returns_array] || op[:has_pagination]
      lines.concat(generate_list_method_body(op, path_expr))
    else
      lines.concat(generate_get_method_body(op, path_expr))
    end

    lines << '      end'
    lines
  end

  def build_params(op)
    params = []

    # Path parameters as keyword args
    op[:path_params].each do |p|
      params << "#{to_snake_case(p[:name])}:"
    end

    # Binary upload parameters
    if op[:has_binary_body]
      params << 'data:'
      params << 'content_type:'
    elsif op[:has_body]
      # Request body parameters as explicit keyword args (not **body)
      # Required body params first (no default), then optional (with nil default)
      required_body_params = op[:body_params].select { |b| b[:required] }
      optional_body_params = op[:body_params].reject { |b| b[:required] }

      required_body_params.each do |b|
        params << "#{to_snake_case(b[:name])}:"
      end

      optional_body_params.each do |b|
        params << "#{to_snake_case(b[:name])}: nil"
      end
    end

    # Query parameters - required first (no default), then optional (with nil default)
    required_query_params = op[:query_params].select { |q| q[:required] }
    optional_query_params = op[:query_params].reject { |q| q[:required] }

    required_query_params.each do |q|
      params << "#{to_snake_case(q[:name])}:"
    end

    optional_query_params.each do |q|
      params << "#{to_snake_case(q[:name])}: nil"
    end

    params.join(', ')
  end

  # Build body hash expression from explicit body params
  def build_body_expression(op)
    return '{}' unless op[:body_params]&.any?

    # Build compact_params call with all body params
    param_mappings = op[:body_params].map do |b|
      ruby_name = to_snake_case(b[:name])
      # Use original API name as key (snake_case), ruby variable as value
      "#{b[:name]}: #{ruby_name}"
    end

    "compact_params(#{param_mappings.join(', ')})"
  end

  def build_path_expression(op)
    path = op[:path]
    # Check if it's a bucket path
    if path.start_with?('/buckets/#{project_id}')
      # Use bucket_path helper - extract the part after /buckets/{project_id}
      path_after_bucket = path.sub(%r{^/buckets/#\{project_id\}}, '')
      "bucket_path(project_id, \"#{path_after_bucket}\")"
    else
      "\"#{path}\""
    end
  end

  def generate_void_method_body(op, path_expr)
    lines = []
    http_method = op[:http_method].downcase

    if op[:has_body]
      body_expr = build_body_expression(op)
      lines << "        http_#{http_method}(#{path_expr}, body: #{body_expr})"
    else
      lines << "        http_#{http_method}(#{path_expr})"
    end
    lines << '        nil'
    lines
  end

  def generate_list_method_body(op, path_expr)
    lines = []

    # Build params hash for query params
    if op[:query_params].any?
      param_names = op[:query_params].map { |q| "#{to_snake_case(q[:name])}: #{to_snake_case(q[:name])}" }
      lines << "        params = compact_params(#{param_names.join(', ')})"
      lines << "        paginate(#{path_expr}, params: params)"
    else
      lines << "        paginate(#{path_expr})"
    end

    lines
  end

  def generate_get_method_body(op, path_expr)
    lines = []
    http_method = op[:http_method].downcase

    if op[:has_binary_body]
      # Binary upload - use raw body and set Content-Type header
      # post_raw accepts (path, body:, content_type:) - no params keyword
      # Query params must be embedded in the URL
      if op[:query_params].any?
        # Build URL with query string
        query_parts = op[:query_params].map { |q| "#{q[:name]}=\#{URI.encode_www_form_component(#{to_snake_case(q[:name])}.to_s)}" }
        query_string = query_parts.join('&')
        # Modify path_expr to include query string
        if path_expr.start_with?('bucket_path')
          # For bucket_path, append query string to the path argument
          path_expr_with_query = path_expr.sub(/\)$/, " + \"?#{query_string}\")")
        else
          # For string paths, append directly
          path_expr_with_query = path_expr.sub(/"$/, "?#{query_string}\"")
        end
        lines << "        http_#{http_method}_raw(#{path_expr_with_query}, body: data, content_type: content_type).json"
      else
        lines << "        http_#{http_method}_raw(#{path_expr}, body: data, content_type: content_type).json"
      end
    elsif op[:has_body]
      body_expr = build_body_expression(op)
      lines << "        http_#{http_method}(#{path_expr}, body: #{body_expr}).json"
    elsif op[:query_params].any?
      param_names = op[:query_params].map { |q| "#{to_snake_case(q[:name])}: #{to_snake_case(q[:name])}" }
      lines << "        http_#{http_method}(#{path_expr}, params: compact_params(#{param_names.join(', ')})).json"
    else
      lines << "        http_#{http_method}(#{path_expr}).json"
    end

    lines
  end
end

# Main execution
if __FILE__ == $PROGRAM_NAME
  openapi_path = nil
  output_dir = nil

  i = 0
  while i < ARGV.length
    case ARGV[i]
    when '--openapi'
      openapi_path = ARGV[i + 1]
      i += 2
    when '--output'
      output_dir = ARGV[i + 1]
      i += 2
    else
      i += 1
    end
  end

  openapi_path ||= File.expand_path('../../openapi.json', __dir__)
  output_dir ||= File.expand_path('../lib/basecamp/generated/services', __dir__)

  unless File.exist?(openapi_path)
    warn "Error: OpenAPI file not found: #{openapi_path}"
    exit 1
  end

  generator = ServiceGenerator.new(openapi_path)
  generator.generate(output_dir)
end
