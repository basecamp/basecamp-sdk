# frozen_string_literal: true

# Auto-generated from OpenAPI spec. Do not edit manually.
# Generated: 2026-02-22T08:10:37Z

require "json"
require "time"

# Type conversion helpers
module TypeHelpers
  module_function

  def identity(value)
    value
  end

  def parse_integer(value)
    return nil if value.nil?
    value.to_i
  end

  def parse_float(value)
    return nil if value.nil?
    value.to_f
  end

  def parse_boolean(value)
    return nil if value.nil?
    !!value
  end

  def parse_datetime(value)
    return nil if value.nil?
    return value if value.is_a?(Time)
    Time.parse(value.to_s)
  rescue ArgumentError
    nil
  end

  def parse_type(value, type_name)
    return nil if value.nil?
    return value unless value.is_a?(Hash)

    type_class = Basecamp::Types.const_get(type_name)
    type_class.new(value)
  rescue NameError
    value
  end

  def parse_array(value, type_name)
    return nil if value.nil?
    return value unless value.is_a?(Array)

    type_class = Basecamp::Types.const_get(type_name)
    value.map { |item| item.is_a?(Hash) ? type_class.new(item) : item }
  rescue NameError
    value
  end
end

module Basecamp
  module Types
    include TypeHelpers

    # Assignable
    class Assignable
      include TypeHelpers
      attr_accessor :app_url, :assignees, :bucket, :due_on, :id, :parent, :starts_on, :title, :type, :url

      def initialize(data = {})
        @app_url = data["app_url"]
        @assignees = parse_array(data["assignees"], "Person")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @due_on = data["due_on"]
        @id = parse_integer(data["id"])
        @parent = parse_type(data["parent"], "TodoParent")
        @starts_on = data["starts_on"]
        @title = data["title"]
        @type = data["type"]
        @url = data["url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "assignees" => @assignees,
          "bucket" => @bucket,
          "due_on" => @due_on,
          "id" => @id,
          "parent" => @parent,
          "starts_on" => @starts_on,
          "title" => @title,
          "type" => @type,
          "url" => @url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Boost
    class Boost
      include TypeHelpers
      attr_accessor :created_at, :id, :booster, :content, :recording

      # @return [Array<Symbol>]
      def self.required_fields
        %i[created_at id].freeze
      end

      def initialize(data = {})
        @created_at = parse_datetime(data["created_at"])
        @id = parse_integer(data["id"])
        @booster = parse_type(data["booster"], "Person")
        @content = data["content"]
        @recording = parse_type(data["recording"], "RecordingParent")
      end

      def to_h
        {
          "created_at" => @created_at,
          "id" => @id,
          "booster" => @booster,
          "content" => @content,
          "recording" => @recording,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Campfire
    class Campfire
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :lines_url, :position, :subscription_url, :topic

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @lines_url = data["lines_url"]
        @position = parse_integer(data["position"])
        @subscription_url = data["subscription_url"]
        @topic = data["topic"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "lines_url" => @lines_url,
          "position" => @position,
          "subscription_url" => @subscription_url,
          "topic" => @topic,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # CampfireLine
    class CampfireLine
      include TypeHelpers
      attr_accessor :app_url, :bucket, :content, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :boosts_count, :boosts_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket content created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @content = data["content"]
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @boosts_count = parse_integer(data["boosts_count"])
        @boosts_url = data["boosts_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "content" => @content,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "boosts_count" => @boosts_count,
          "boosts_url" => @boosts_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Card
    class Card
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :assignees, :bookmark_url, :boosts_count, :boosts_url, :comments_count, :comments_url, :completed, :completed_at, :completer, :completion_subscribers, :completion_url, :content, :description, :due_on, :position, :steps, :subscription_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @assignees = parse_array(data["assignees"], "Person")
        @bookmark_url = data["bookmark_url"]
        @boosts_count = parse_integer(data["boosts_count"])
        @boosts_url = data["boosts_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @completed = parse_boolean(data["completed"])
        @completed_at = parse_datetime(data["completed_at"])
        @completer = parse_type(data["completer"], "Person")
        @completion_subscribers = parse_array(data["completion_subscribers"], "Person")
        @completion_url = data["completion_url"]
        @content = data["content"]
        @description = data["description"]
        @due_on = data["due_on"]
        @position = parse_integer(data["position"])
        @steps = parse_array(data["steps"], "CardStep")
        @subscription_url = data["subscription_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "assignees" => @assignees,
          "bookmark_url" => @bookmark_url,
          "boosts_count" => @boosts_count,
          "boosts_url" => @boosts_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "completed" => @completed,
          "completed_at" => @completed_at,
          "completer" => @completer,
          "completion_subscribers" => @completion_subscribers,
          "completion_url" => @completion_url,
          "content" => @content,
          "description" => @description,
          "due_on" => @due_on,
          "position" => @position,
          "steps" => @steps,
          "subscription_url" => @subscription_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # CardColumn
    class CardColumn
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :cards_count, :cards_url, :color, :comments_count, :description, :position, :subscribers

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @cards_count = parse_integer(data["cards_count"])
        @cards_url = data["cards_url"]
        @color = data["color"]
        @comments_count = parse_integer(data["comments_count"])
        @description = data["description"]
        @position = parse_integer(data["position"])
        @subscribers = parse_array(data["subscribers"], "Person")
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "cards_count" => @cards_count,
          "cards_url" => @cards_url,
          "color" => @color,
          "comments_count" => @comments_count,
          "description" => @description,
          "position" => @position,
          "subscribers" => @subscribers,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # CardStep
    class CardStep
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :assignees, :bookmark_url, :completed, :completed_at, :completer, :completion_url, :due_on, :position

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @assignees = parse_array(data["assignees"], "Person")
        @bookmark_url = data["bookmark_url"]
        @completed = parse_boolean(data["completed"])
        @completed_at = parse_datetime(data["completed_at"])
        @completer = parse_type(data["completer"], "Person")
        @completion_url = data["completion_url"]
        @due_on = data["due_on"]
        @position = parse_integer(data["position"])
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "assignees" => @assignees,
          "bookmark_url" => @bookmark_url,
          "completed" => @completed,
          "completed_at" => @completed_at,
          "completer" => @completer,
          "completion_url" => @completion_url,
          "due_on" => @due_on,
          "position" => @position,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # CardTable
    class CardTable
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :lists, :subscribers, :subscription_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @lists = parse_array(data["lists"], "CardColumn")
        @subscribers = parse_array(data["subscribers"], "Person")
        @subscription_url = data["subscription_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "lists" => @lists,
          "subscribers" => @subscribers,
          "subscription_url" => @subscription_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Chatbot
    class Chatbot
      include TypeHelpers
      attr_accessor :created_at, :id, :service_name, :updated_at, :app_url, :command_url, :lines_url, :url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[created_at id service_name updated_at].freeze
      end

      def initialize(data = {})
        @created_at = parse_datetime(data["created_at"])
        @id = parse_integer(data["id"])
        @service_name = data["service_name"]
        @updated_at = parse_datetime(data["updated_at"])
        @app_url = data["app_url"]
        @command_url = data["command_url"]
        @lines_url = data["lines_url"]
        @url = data["url"]
      end

      def to_h
        {
          "created_at" => @created_at,
          "id" => @id,
          "service_name" => @service_name,
          "updated_at" => @updated_at,
          "app_url" => @app_url,
          "command_url" => @command_url,
          "lines_url" => @lines_url,
          "url" => @url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ClientApproval
    class ClientApproval
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :approval_status, :approver, :bookmark_url, :content, :due_on, :replies_count, :replies_url, :responses, :subject, :subscription_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @approval_status = data["approval_status"]
        @approver = parse_type(data["approver"], "Person")
        @bookmark_url = data["bookmark_url"]
        @content = data["content"]
        @due_on = data["due_on"]
        @replies_count = parse_integer(data["replies_count"])
        @replies_url = data["replies_url"]
        @responses = parse_array(data["responses"], "ClientApprovalResponse")
        @subject = data["subject"]
        @subscription_url = data["subscription_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "approval_status" => @approval_status,
          "approver" => @approver,
          "bookmark_url" => @bookmark_url,
          "content" => @content,
          "due_on" => @due_on,
          "replies_count" => @replies_count,
          "replies_url" => @replies_url,
          "responses" => @responses,
          "subject" => @subject,
          "subscription_url" => @subscription_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ClientApprovalResponse
    class ClientApprovalResponse
      include TypeHelpers
      attr_accessor :app_url, :approved, :bookmark_url, :bucket, :content, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :visible_to_clients

      def initialize(data = {})
        @app_url = data["app_url"]
        @approved = parse_boolean(data["approved"])
        @bookmark_url = data["bookmark_url"]
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @content = data["content"]
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
      end

      def to_h
        {
          "app_url" => @app_url,
          "approved" => @approved,
          "bookmark_url" => @bookmark_url,
          "bucket" => @bucket,
          "content" => @content,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "visible_to_clients" => @visible_to_clients,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ClientCompany
    class ClientCompany
      include TypeHelpers
      attr_accessor :id, :name

      # @return [Array<Symbol>]
      def self.required_fields
        %i[id name].freeze
      end

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @name = data["name"]
      end

      def to_h
        {
          "id" => @id,
          "name" => @name,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ClientCorrespondence
    class ClientCorrespondence
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :parent, :status, :subject, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :content, :replies_count, :replies_url, :subscription_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status parent status subject title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @subject = data["subject"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @content = data["content"]
        @replies_count = parse_integer(data["replies_count"])
        @replies_url = data["replies_url"]
        @subscription_url = data["subscription_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "subject" => @subject,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "content" => @content,
          "replies_count" => @replies_count,
          "replies_url" => @replies_url,
          "subscription_url" => @subscription_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ClientReply
    class ClientReply
      include TypeHelpers
      attr_accessor :app_url, :bucket, :content, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket content created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @content = data["content"]
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "content" => @content,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ClientSide
    class ClientSide
      include TypeHelpers
      attr_accessor :app_url, :url

      def initialize(data = {})
        @app_url = data["app_url"]
        @url = data["url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "url" => @url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Comment
    class Comment
      include TypeHelpers
      attr_accessor :app_url, :bucket, :content, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :boosts_count, :boosts_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket content created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @content = data["content"]
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @boosts_count = parse_integer(data["boosts_count"])
        @boosts_url = data["boosts_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "content" => @content,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "boosts_count" => @boosts_count,
          "boosts_url" => @boosts_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # CreatePersonRequest
    class CreatePersonRequest
      include TypeHelpers
      attr_accessor :email_address, :name, :company_name, :title

      # @return [Array<Symbol>]
      def self.required_fields
        %i[email_address name].freeze
      end

      def initialize(data = {})
        @email_address = data["email_address"]
        @name = data["name"]
        @company_name = data["company_name"]
        @title = data["title"]
      end

      def to_h
        {
          "email_address" => @email_address,
          "name" => @name,
          "company_name" => @company_name,
          "title" => @title,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # DockItem
    class DockItem
      include TypeHelpers
      attr_accessor :app_url, :enabled, :id, :name, :title, :url, :position

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url enabled id name title url].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @enabled = parse_boolean(data["enabled"])
        @id = parse_integer(data["id"])
        @name = data["name"]
        @title = data["title"]
        @url = data["url"]
        @position = parse_integer(data["position"])
      end

      def to_h
        {
          "app_url" => @app_url,
          "enabled" => @enabled,
          "id" => @id,
          "name" => @name,
          "title" => @title,
          "url" => @url,
          "position" => @position,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Document
    class Document
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :boosts_count, :boosts_url, :comments_count, :comments_url, :content, :position, :subscription_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @boosts_count = parse_integer(data["boosts_count"])
        @boosts_url = data["boosts_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @content = data["content"]
        @position = parse_integer(data["position"])
        @subscription_url = data["subscription_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "boosts_count" => @boosts_count,
          "boosts_url" => @boosts_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "content" => @content,
          "position" => @position,
          "subscription_url" => @subscription_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Event
    class Event
      include TypeHelpers
      attr_accessor :action, :created_at, :creator, :id, :recording_id, :boosts_count, :boosts_url, :details

      # @return [Array<Symbol>]
      def self.required_fields
        %i[action created_at creator id recording_id].freeze
      end

      def initialize(data = {})
        @action = data["action"]
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @recording_id = parse_integer(data["recording_id"])
        @boosts_count = parse_integer(data["boosts_count"])
        @boosts_url = data["boosts_url"]
        @details = parse_type(data["details"], "EventDetails")
      end

      def to_h
        {
          "action" => @action,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "recording_id" => @recording_id,
          "boosts_count" => @boosts_count,
          "boosts_url" => @boosts_url,
          "details" => @details,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # EventDetails
    class EventDetails
      include TypeHelpers
      attr_accessor :added_person_ids, :notified_recipient_ids, :removed_person_ids

      def initialize(data = {})
        @added_person_ids = data["added_person_ids"]
        @notified_recipient_ids = data["notified_recipient_ids"]
        @removed_person_ids = data["removed_person_ids"]
      end

      def to_h
        {
          "added_person_ids" => @added_person_ids,
          "notified_recipient_ids" => @notified_recipient_ids,
          "removed_person_ids" => @removed_person_ids,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Forward
    class Forward
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :parent, :status, :subject, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :content, :from, :replies_count, :replies_url, :subscription_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status parent status subject title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @subject = data["subject"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @content = data["content"]
        @from = data["from"]
        @replies_count = parse_integer(data["replies_count"])
        @replies_url = data["replies_url"]
        @subscription_url = data["subscription_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "subject" => @subject,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "content" => @content,
          "from" => @from,
          "replies_count" => @replies_count,
          "replies_url" => @replies_url,
          "subscription_url" => @subscription_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ForwardReply
    class ForwardReply
      include TypeHelpers
      attr_accessor :app_url, :bucket, :content, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :boosts_count, :boosts_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket content created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @content = data["content"]
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @boosts_count = parse_integer(data["boosts_count"])
        @boosts_url = data["boosts_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "content" => @content,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "boosts_count" => @boosts_count,
          "boosts_url" => @boosts_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Inbox
    class Inbox
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :forwards_count, :forwards_url, :position

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @forwards_count = parse_integer(data["forwards_count"])
        @forwards_url = data["forwards_url"]
        @position = parse_integer(data["position"])
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "forwards_count" => @forwards_count,
          "forwards_url" => @forwards_url,
          "position" => @position,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Message
    class Message
      include TypeHelpers
      attr_accessor :app_url, :bucket, :content, :created_at, :creator, :id, :inherits_status, :parent, :status, :subject, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :boosts_count, :boosts_url, :category, :comments_count, :comments_url, :subscription_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket content created_at creator id inherits_status parent status subject title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @content = data["content"]
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @subject = data["subject"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @boosts_count = parse_integer(data["boosts_count"])
        @boosts_url = data["boosts_url"]
        @category = parse_type(data["category"], "MessageType")
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @subscription_url = data["subscription_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "content" => @content,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "subject" => @subject,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "boosts_count" => @boosts_count,
          "boosts_url" => @boosts_url,
          "category" => @category,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "subscription_url" => @subscription_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # MessageBoard
    class MessageBoard
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :status, :title, :type, :updated_at, :url, :visible_to_clients, :app_messages_url, :bookmark_url, :messages_count, :messages_url, :position

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @app_messages_url = data["app_messages_url"]
        @bookmark_url = data["bookmark_url"]
        @messages_count = parse_integer(data["messages_count"])
        @messages_url = data["messages_url"]
        @position = parse_integer(data["position"])
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "app_messages_url" => @app_messages_url,
          "bookmark_url" => @bookmark_url,
          "messages_count" => @messages_count,
          "messages_url" => @messages_url,
          "position" => @position,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # MessageType
    class MessageType
      include TypeHelpers
      attr_accessor :created_at, :icon, :id, :name, :updated_at

      # @return [Array<Symbol>]
      def self.required_fields
        %i[created_at icon id name updated_at].freeze
      end

      def initialize(data = {})
        @created_at = parse_datetime(data["created_at"])
        @icon = data["icon"]
        @id = parse_integer(data["id"])
        @name = data["name"]
        @updated_at = parse_datetime(data["updated_at"])
      end

      def to_h
        {
          "created_at" => @created_at,
          "icon" => @icon,
          "id" => @id,
          "name" => @name,
          "updated_at" => @updated_at,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Person
    class Person
      include TypeHelpers
      attr_accessor :id, :name, :admin, :attachable_sgid, :avatar_url, :bio, :can_access_hill_charts, :can_access_timesheet, :can_manage_people, :can_manage_projects, :can_ping, :client, :company, :created_at, :email_address, :employee, :location, :owner, :personable_type, :time_zone, :title, :updated_at

      # @return [Array<Symbol>]
      def self.required_fields
        %i[id name].freeze
      end

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @name = data["name"]
        @admin = parse_boolean(data["admin"])
        @attachable_sgid = data["attachable_sgid"]
        @avatar_url = data["avatar_url"]
        @bio = data["bio"]
        @can_access_hill_charts = parse_boolean(data["can_access_hill_charts"])
        @can_access_timesheet = parse_boolean(data["can_access_timesheet"])
        @can_manage_people = parse_boolean(data["can_manage_people"])
        @can_manage_projects = parse_boolean(data["can_manage_projects"])
        @can_ping = parse_boolean(data["can_ping"])
        @client = parse_boolean(data["client"])
        @company = parse_type(data["company"], "PersonCompany")
        @created_at = parse_datetime(data["created_at"])
        @email_address = data["email_address"]
        @employee = parse_boolean(data["employee"])
        @location = data["location"]
        @owner = parse_boolean(data["owner"])
        @personable_type = data["personable_type"]
        @time_zone = data["time_zone"]
        @title = data["title"]
        @updated_at = parse_datetime(data["updated_at"])
      end

      def to_h
        {
          "id" => @id,
          "name" => @name,
          "admin" => @admin,
          "attachable_sgid" => @attachable_sgid,
          "avatar_url" => @avatar_url,
          "bio" => @bio,
          "can_access_hill_charts" => @can_access_hill_charts,
          "can_access_timesheet" => @can_access_timesheet,
          "can_manage_people" => @can_manage_people,
          "can_manage_projects" => @can_manage_projects,
          "can_ping" => @can_ping,
          "client" => @client,
          "company" => @company,
          "created_at" => @created_at,
          "email_address" => @email_address,
          "employee" => @employee,
          "location" => @location,
          "owner" => @owner,
          "personable_type" => @personable_type,
          "time_zone" => @time_zone,
          "title" => @title,
          "updated_at" => @updated_at,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # PersonCompany
    class PersonCompany
      include TypeHelpers
      attr_accessor :id, :name

      # @return [Array<Symbol>]
      def self.required_fields
        %i[id name].freeze
      end

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @name = data["name"]
      end

      def to_h
        {
          "id" => @id,
          "name" => @name,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Project
    class Project
      include TypeHelpers
      attr_accessor :app_url, :created_at, :id, :name, :status, :updated_at, :url, :bookmark_url, :bookmarked, :client_company, :clients_enabled, :clientside, :description, :dock, :purpose

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url created_at id name status updated_at url].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @created_at = parse_datetime(data["created_at"])
        @id = parse_integer(data["id"])
        @name = data["name"]
        @status = data["status"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @bookmark_url = data["bookmark_url"]
        @bookmarked = parse_boolean(data["bookmarked"])
        @client_company = parse_type(data["client_company"], "ClientCompany")
        @clients_enabled = parse_boolean(data["clients_enabled"])
        @clientside = parse_type(data["clientside"], "ClientSide")
        @description = data["description"]
        @dock = parse_array(data["dock"], "DockItem")
        @purpose = data["purpose"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "created_at" => @created_at,
          "id" => @id,
          "name" => @name,
          "status" => @status,
          "updated_at" => @updated_at,
          "url" => @url,
          "bookmark_url" => @bookmark_url,
          "bookmarked" => @bookmarked,
          "client_company" => @client_company,
          "clients_enabled" => @clients_enabled,
          "clientside" => @clientside,
          "description" => @description,
          "dock" => @dock,
          "purpose" => @purpose,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ProjectAccessResult
    class ProjectAccessResult
      include TypeHelpers
      attr_accessor :granted, :revoked

      def initialize(data = {})
        @granted = parse_array(data["granted"], "Person")
        @revoked = parse_array(data["revoked"], "Person")
      end

      def to_h
        {
          "granted" => @granted,
          "revoked" => @revoked,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ProjectConstruction
    class ProjectConstruction
      include TypeHelpers
      attr_accessor :id, :status, :project, :url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[id status].freeze
      end

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @project = parse_type(data["project"], "Project")
        @url = data["url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "project" => @project,
          "url" => @url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Question
    class Question
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :answers_count, :answers_url, :bookmark_url, :paused, :schedule, :subscription_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @answers_count = parse_integer(data["answers_count"])
        @answers_url = data["answers_url"]
        @bookmark_url = data["bookmark_url"]
        @paused = parse_boolean(data["paused"])
        @schedule = parse_type(data["schedule"], "QuestionSchedule")
        @subscription_url = data["subscription_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "answers_count" => @answers_count,
          "answers_url" => @answers_url,
          "bookmark_url" => @bookmark_url,
          "paused" => @paused,
          "schedule" => @schedule,
          "subscription_url" => @subscription_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # QuestionAnswer
    class QuestionAnswer
      include TypeHelpers
      attr_accessor :app_url, :bucket, :content, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :boosts_count, :boosts_url, :comments_count, :comments_url, :group_on, :subscription_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket content created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @content = data["content"]
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @boosts_count = parse_integer(data["boosts_count"])
        @boosts_url = data["boosts_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @group_on = data["group_on"]
        @subscription_url = data["subscription_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "content" => @content,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "boosts_count" => @boosts_count,
          "boosts_url" => @boosts_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "group_on" => @group_on,
          "subscription_url" => @subscription_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # QuestionAnswerPayload
    class QuestionAnswerPayload
      include TypeHelpers
      attr_accessor :content, :group_on

      # @return [Array<Symbol>]
      def self.required_fields
        %i[content].freeze
      end

      def initialize(data = {})
        @content = data["content"]
        @group_on = data["group_on"]
      end

      def to_h
        {
          "content" => @content,
          "group_on" => @group_on,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # QuestionAnswerUpdatePayload
    class QuestionAnswerUpdatePayload
      include TypeHelpers
      attr_accessor :content

      # @return [Array<Symbol>]
      def self.required_fields
        %i[content].freeze
      end

      def initialize(data = {})
        @content = data["content"]
      end

      def to_h
        {
          "content" => @content,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # QuestionReminder
    class QuestionReminder
      include TypeHelpers
      attr_accessor :group_on, :question, :remind_at, :reminder_id

      def initialize(data = {})
        @group_on = data["group_on"]
        @question = parse_type(data["question"], "Question")
        @remind_at = parse_datetime(data["remind_at"])
        @reminder_id = parse_integer(data["reminder_id"])
      end

      def to_h
        {
          "group_on" => @group_on,
          "question" => @question,
          "remind_at" => @remind_at,
          "reminder_id" => @reminder_id,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # QuestionSchedule
    class QuestionSchedule
      include TypeHelpers
      attr_accessor :days, :end_date, :frequency, :hour, :minute, :month_interval, :start_date, :week_instance, :week_interval

      def initialize(data = {})
        @days = data["days"]
        @end_date = data["end_date"]
        @frequency = data["frequency"]
        @hour = parse_integer(data["hour"])
        @minute = parse_integer(data["minute"])
        @month_interval = parse_integer(data["month_interval"])
        @start_date = data["start_date"]
        @week_instance = parse_integer(data["week_instance"])
        @week_interval = parse_integer(data["week_interval"])
      end

      def to_h
        {
          "days" => @days,
          "end_date" => @end_date,
          "frequency" => @frequency,
          "hour" => @hour,
          "minute" => @minute,
          "month_interval" => @month_interval,
          "start_date" => @start_date,
          "week_instance" => @week_instance,
          "week_interval" => @week_interval,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Questionnaire
    class Questionnaire
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :name, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :questions_count, :questions_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status name status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @name = data["name"]
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @questions_count = parse_integer(data["questions_count"])
        @questions_url = data["questions_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "name" => @name,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "questions_count" => @questions_count,
          "questions_url" => @questions_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Recording
    class Recording
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :comments_count, :comments_url, :content, :subscription_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @content = data["content"]
        @subscription_url = data["subscription_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "content" => @content,
          "subscription_url" => @subscription_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # RecordingBucket
    class RecordingBucket
      include TypeHelpers
      attr_accessor :id, :name, :type

      # @return [Array<Symbol>]
      def self.required_fields
        %i[id name type].freeze
      end

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @name = data["name"]
        @type = data["type"]
      end

      def to_h
        {
          "id" => @id,
          "name" => @name,
          "type" => @type,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # RecordingParent
    class RecordingParent
      include TypeHelpers
      attr_accessor :app_url, :id, :title, :type, :url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url id title type url].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @id = parse_integer(data["id"])
        @title = data["title"]
        @type = data["type"]
        @url = data["url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "id" => @id,
          "title" => @title,
          "type" => @type,
          "url" => @url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Schedule
    class Schedule
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :entries_count, :entries_url, :include_due_assignments, :position

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @entries_count = parse_integer(data["entries_count"])
        @entries_url = data["entries_url"]
        @include_due_assignments = parse_boolean(data["include_due_assignments"])
        @position = parse_integer(data["position"])
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "entries_count" => @entries_count,
          "entries_url" => @entries_url,
          "include_due_assignments" => @include_due_assignments,
          "position" => @position,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ScheduleAttributes
    class ScheduleAttributes
      include TypeHelpers
      attr_accessor :end_date, :start_date

      def initialize(data = {})
        @end_date = data["end_date"]
        @start_date = data["start_date"]
      end

      def to_h
        {
          "end_date" => @end_date,
          "start_date" => @start_date,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ScheduleEntry
    class ScheduleEntry
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :parent, :status, :summary, :title, :type, :updated_at, :url, :visible_to_clients, :all_day, :bookmark_url, :boosts_count, :boosts_url, :comments_count, :comments_url, :description, :ends_at, :participants, :starts_at, :subscription_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status parent status summary title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @summary = data["summary"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @all_day = parse_boolean(data["all_day"])
        @bookmark_url = data["bookmark_url"]
        @boosts_count = parse_integer(data["boosts_count"])
        @boosts_url = data["boosts_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @description = data["description"]
        @ends_at = parse_datetime(data["ends_at"])
        @participants = parse_array(data["participants"], "Person")
        @starts_at = parse_datetime(data["starts_at"])
        @subscription_url = data["subscription_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "summary" => @summary,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "all_day" => @all_day,
          "bookmark_url" => @bookmark_url,
          "boosts_count" => @boosts_count,
          "boosts_url" => @boosts_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "description" => @description,
          "ends_at" => @ends_at,
          "participants" => @participants,
          "starts_at" => @starts_at,
          "subscription_url" => @subscription_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # SearchMetadata
    class SearchMetadata
      include TypeHelpers
      attr_accessor :projects

      def initialize(data = {})
        @projects = parse_array(data["projects"], "SearchProject")
      end

      def to_h
        {
          "projects" => @projects,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # SearchProject
    class SearchProject
      include TypeHelpers
      attr_accessor :id, :name

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @name = data["name"]
      end

      def to_h
        {
          "id" => @id,
          "name" => @name,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # SearchResult
    class SearchResult
      include TypeHelpers
      attr_accessor :app_url, :id, :title, :type, :url, :bookmark_url, :bucket, :content, :created_at, :creator, :description, :inherits_status, :parent, :status, :subject, :updated_at, :visible_to_clients

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url id title type url].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @id = parse_integer(data["id"])
        @title = data["title"]
        @type = data["type"]
        @url = data["url"]
        @bookmark_url = data["bookmark_url"]
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @content = data["content"]
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @description = data["description"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @subject = data["subject"]
        @updated_at = parse_datetime(data["updated_at"])
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
      end

      def to_h
        {
          "app_url" => @app_url,
          "id" => @id,
          "title" => @title,
          "type" => @type,
          "url" => @url,
          "bookmark_url" => @bookmark_url,
          "bucket" => @bucket,
          "content" => @content,
          "created_at" => @created_at,
          "creator" => @creator,
          "description" => @description,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "subject" => @subject,
          "updated_at" => @updated_at,
          "visible_to_clients" => @visible_to_clients,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Subscription
    class Subscription
      include TypeHelpers
      attr_accessor :count, :subscribed, :url, :subscribers

      # @return [Array<Symbol>]
      def self.required_fields
        %i[count subscribed url].freeze
      end

      def initialize(data = {})
        @count = parse_integer(data["count"])
        @subscribed = parse_boolean(data["subscribed"])
        @url = data["url"]
        @subscribers = parse_array(data["subscribers"], "Person")
      end

      def to_h
        {
          "count" => @count,
          "subscribed" => @subscribed,
          "url" => @url,
          "subscribers" => @subscribers,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Template
    class Template
      include TypeHelpers
      attr_accessor :created_at, :id, :name, :updated_at, :app_url, :description, :dock, :status, :url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[created_at id name updated_at].freeze
      end

      def initialize(data = {})
        @created_at = parse_datetime(data["created_at"])
        @id = parse_integer(data["id"])
        @name = data["name"]
        @updated_at = parse_datetime(data["updated_at"])
        @app_url = data["app_url"]
        @description = data["description"]
        @dock = parse_array(data["dock"], "DockItem")
        @status = data["status"]
        @url = data["url"]
      end

      def to_h
        {
          "created_at" => @created_at,
          "id" => @id,
          "name" => @name,
          "updated_at" => @updated_at,
          "app_url" => @app_url,
          "description" => @description,
          "dock" => @dock,
          "status" => @status,
          "url" => @url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # TimelineEvent
    class TimelineEvent
      include TypeHelpers
      attr_accessor :action, :app_url, :bucket, :created_at, :creator, :id, :kind, :parent_recording_id, :summary_excerpt, :target, :title, :url

      def initialize(data = {})
        @action = data["action"]
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @kind = data["kind"]
        @parent_recording_id = parse_integer(data["parent_recording_id"])
        @summary_excerpt = data["summary_excerpt"]
        @target = data["target"]
        @title = data["title"]
        @url = data["url"]
      end

      def to_h
        {
          "action" => @action,
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "kind" => @kind,
          "parent_recording_id" => @parent_recording_id,
          "summary_excerpt" => @summary_excerpt,
          "target" => @target,
          "title" => @title,
          "url" => @url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # TimesheetEntry
    class TimesheetEntry
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :date, :description, :hours, :person

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @date = data["date"]
        @description = data["description"]
        @hours = data["hours"]
        @person = parse_type(data["person"], "Person")
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "date" => @date,
          "description" => @description,
          "hours" => @hours,
          "person" => @person,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Todo
    class Todo
      include TypeHelpers
      attr_accessor :app_url, :bucket, :content, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :assignees, :bookmark_url, :boosts_count, :boosts_url, :comments_count, :comments_url, :completed, :completion_subscribers, :completion_url, :description, :due_on, :position, :starts_on, :subscription_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket content created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @content = data["content"]
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "TodoParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @assignees = parse_array(data["assignees"], "Person")
        @bookmark_url = data["bookmark_url"]
        @boosts_count = parse_integer(data["boosts_count"])
        @boosts_url = data["boosts_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @completed = parse_boolean(data["completed"])
        @completion_subscribers = parse_array(data["completion_subscribers"], "Person")
        @completion_url = data["completion_url"]
        @description = data["description"]
        @due_on = data["due_on"]
        @position = parse_integer(data["position"])
        @starts_on = data["starts_on"]
        @subscription_url = data["subscription_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "content" => @content,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "assignees" => @assignees,
          "bookmark_url" => @bookmark_url,
          "boosts_count" => @boosts_count,
          "boosts_url" => @boosts_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "completed" => @completed,
          "completion_subscribers" => @completion_subscribers,
          "completion_url" => @completion_url,
          "description" => @description,
          "due_on" => @due_on,
          "position" => @position,
          "starts_on" => @starts_on,
          "subscription_url" => @subscription_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # TodoBucket
    class TodoBucket
      include TypeHelpers
      attr_accessor :id, :name, :type

      # @return [Array<Symbol>]
      def self.required_fields
        %i[id name type].freeze
      end

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @name = data["name"]
        @type = data["type"]
      end

      def to_h
        {
          "id" => @id,
          "name" => @name,
          "type" => @type,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # TodoParent
    class TodoParent
      include TypeHelpers
      attr_accessor :app_url, :id, :title, :type, :url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url id title type url].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @id = parse_integer(data["id"])
        @title = data["title"]
        @type = data["type"]
        @url = data["url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "id" => @id,
          "title" => @title,
          "type" => @type,
          "url" => @url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Todolist
    class Todolist
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :name, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :app_todos_url, :bookmark_url, :boosts_count, :boosts_url, :comments_count, :comments_url, :completed, :completed_ratio, :description, :groups_url, :position, :subscription_url, :todos_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status name parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @name = data["name"]
        @parent = parse_type(data["parent"], "TodoParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @app_todos_url = data["app_todos_url"]
        @bookmark_url = data["bookmark_url"]
        @boosts_count = parse_integer(data["boosts_count"])
        @boosts_url = data["boosts_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @completed = parse_boolean(data["completed"])
        @completed_ratio = data["completed_ratio"]
        @description = data["description"]
        @groups_url = data["groups_url"]
        @position = parse_integer(data["position"])
        @subscription_url = data["subscription_url"]
        @todos_url = data["todos_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "name" => @name,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "app_todos_url" => @app_todos_url,
          "bookmark_url" => @bookmark_url,
          "boosts_count" => @boosts_count,
          "boosts_url" => @boosts_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "completed" => @completed,
          "completed_ratio" => @completed_ratio,
          "description" => @description,
          "groups_url" => @groups_url,
          "position" => @position,
          "subscription_url" => @subscription_url,
          "todos_url" => @todos_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # TodolistGroup
    class TodolistGroup
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :name, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :app_todos_url, :bookmark_url, :comments_count, :comments_url, :completed, :completed_ratio, :position, :subscription_url, :todos_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status name parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @name = data["name"]
        @parent = parse_type(data["parent"], "TodoParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @app_todos_url = data["app_todos_url"]
        @bookmark_url = data["bookmark_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @completed = parse_boolean(data["completed"])
        @completed_ratio = data["completed_ratio"]
        @position = parse_integer(data["position"])
        @subscription_url = data["subscription_url"]
        @todos_url = data["todos_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "name" => @name,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "app_todos_url" => @app_todos_url,
          "bookmark_url" => @bookmark_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "completed" => @completed,
          "completed_ratio" => @completed_ratio,
          "position" => @position,
          "subscription_url" => @subscription_url,
          "todos_url" => @todos_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Todoset
    class Todoset
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :name, :status, :title, :type, :updated_at, :url, :visible_to_clients, :app_todolists_url, :bookmark_url, :completed, :completed_count, :completed_ratio, :on_schedule_count, :over_schedule_count, :position, :todolists_count, :todolists_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status name status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @name = data["name"]
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @app_todolists_url = data["app_todolists_url"]
        @bookmark_url = data["bookmark_url"]
        @completed = parse_boolean(data["completed"])
        @completed_count = parse_integer(data["completed_count"])
        @completed_ratio = data["completed_ratio"]
        @on_schedule_count = parse_integer(data["on_schedule_count"])
        @over_schedule_count = parse_integer(data["over_schedule_count"])
        @position = parse_integer(data["position"])
        @todolists_count = parse_integer(data["todolists_count"])
        @todolists_url = data["todolists_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "name" => @name,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "app_todolists_url" => @app_todolists_url,
          "bookmark_url" => @bookmark_url,
          "completed" => @completed,
          "completed_count" => @completed_count,
          "completed_ratio" => @completed_ratio,
          "on_schedule_count" => @on_schedule_count,
          "over_schedule_count" => @over_schedule_count,
          "position" => @position,
          "todolists_count" => @todolists_count,
          "todolists_url" => @todolists_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Tool
    class Tool
      include TypeHelpers
      attr_accessor :created_at, :enabled, :id, :name, :title, :updated_at, :app_url, :bucket, :position, :status, :url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[created_at enabled id name title updated_at].freeze
      end

      def initialize(data = {})
        @created_at = parse_datetime(data["created_at"])
        @enabled = parse_boolean(data["enabled"])
        @id = parse_integer(data["id"])
        @name = data["name"]
        @title = data["title"]
        @updated_at = parse_datetime(data["updated_at"])
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @position = parse_integer(data["position"])
        @status = data["status"]
        @url = data["url"]
      end

      def to_h
        {
          "created_at" => @created_at,
          "enabled" => @enabled,
          "id" => @id,
          "name" => @name,
          "title" => @title,
          "updated_at" => @updated_at,
          "app_url" => @app_url,
          "bucket" => @bucket,
          "position" => @position,
          "status" => @status,
          "url" => @url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Upload
    class Upload
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :parent, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :boosts_count, :boosts_url, :byte_size, :comments_count, :comments_url, :content_type, :description, :download_url, :filename, :height, :position, :subscription_url, :width

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status parent status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @boosts_count = parse_integer(data["boosts_count"])
        @boosts_url = data["boosts_url"]
        @byte_size = parse_integer(data["byte_size"])
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @content_type = data["content_type"]
        @description = data["description"]
        @download_url = data["download_url"]
        @filename = data["filename"]
        @height = parse_integer(data["height"])
        @position = parse_integer(data["position"])
        @subscription_url = data["subscription_url"]
        @width = parse_integer(data["width"])
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "parent" => @parent,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "boosts_count" => @boosts_count,
          "boosts_url" => @boosts_url,
          "byte_size" => @byte_size,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "content_type" => @content_type,
          "description" => @description,
          "download_url" => @download_url,
          "filename" => @filename,
          "height" => @height,
          "position" => @position,
          "subscription_url" => @subscription_url,
          "width" => @width,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Vault
    class Vault
      include TypeHelpers
      attr_accessor :app_url, :bucket, :created_at, :creator, :id, :inherits_status, :status, :title, :type, :updated_at, :url, :visible_to_clients, :bookmark_url, :documents_count, :documents_url, :parent, :position, :uploads_count, :uploads_url, :vaults_count, :vaults_url

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url bucket created_at creator id inherits_status status title type updated_at url visible_to_clients].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @id = parse_integer(data["id"])
        @inherits_status = parse_boolean(data["inherits_status"])
        @status = data["status"]
        @title = data["title"]
        @type = data["type"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @bookmark_url = data["bookmark_url"]
        @documents_count = parse_integer(data["documents_count"])
        @documents_url = data["documents_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @position = parse_integer(data["position"])
        @uploads_count = parse_integer(data["uploads_count"])
        @uploads_url = data["uploads_url"]
        @vaults_count = parse_integer(data["vaults_count"])
        @vaults_url = data["vaults_url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "created_at" => @created_at,
          "creator" => @creator,
          "id" => @id,
          "inherits_status" => @inherits_status,
          "status" => @status,
          "title" => @title,
          "type" => @type,
          "updated_at" => @updated_at,
          "url" => @url,
          "visible_to_clients" => @visible_to_clients,
          "bookmark_url" => @bookmark_url,
          "documents_count" => @documents_count,
          "documents_url" => @documents_url,
          "parent" => @parent,
          "position" => @position,
          "uploads_count" => @uploads_count,
          "uploads_url" => @uploads_url,
          "vaults_count" => @vaults_count,
          "vaults_url" => @vaults_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Webhook
    class Webhook
      include TypeHelpers
      attr_accessor :app_url, :created_at, :id, :payload_url, :updated_at, :url, :active, :recent_deliveries, :types

      # @return [Array<Symbol>]
      def self.required_fields
        %i[app_url created_at id payload_url updated_at url].freeze
      end

      def initialize(data = {})
        @app_url = data["app_url"]
        @created_at = parse_datetime(data["created_at"])
        @id = parse_integer(data["id"])
        @payload_url = data["payload_url"]
        @updated_at = parse_datetime(data["updated_at"])
        @url = data["url"]
        @active = parse_boolean(data["active"])
        @recent_deliveries = parse_array(data["recent_deliveries"], "WebhookDelivery")
        @types = data["types"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "created_at" => @created_at,
          "id" => @id,
          "payload_url" => @payload_url,
          "updated_at" => @updated_at,
          "url" => @url,
          "active" => @active,
          "recent_deliveries" => @recent_deliveries,
          "types" => @types,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # WebhookCopy
    class WebhookCopy
      include TypeHelpers
      attr_accessor :app_url, :bucket, :id, :url

      def initialize(data = {})
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "WebhookCopyBucket")
        @id = parse_integer(data["id"])
        @url = data["url"]
      end

      def to_h
        {
          "app_url" => @app_url,
          "bucket" => @bucket,
          "id" => @id,
          "url" => @url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # WebhookCopyBucket
    class WebhookCopyBucket
      include TypeHelpers
      attr_accessor :id

      def initialize(data = {})
        @id = parse_integer(data["id"])
      end

      def to_h
        {
          "id" => @id,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # WebhookDelivery
    class WebhookDelivery
      include TypeHelpers
      attr_accessor :created_at, :id, :request, :response

      def initialize(data = {})
        @created_at = parse_datetime(data["created_at"])
        @id = parse_integer(data["id"])
        @request = parse_type(data["request"], "WebhookDeliveryRequest")
        @response = parse_type(data["response"], "WebhookDeliveryResponse")
      end

      def to_h
        {
          "created_at" => @created_at,
          "id" => @id,
          "request" => @request,
          "response" => @response,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # WebhookDeliveryRequest
    class WebhookDeliveryRequest
      include TypeHelpers
      attr_accessor :body, :headers

      def initialize(data = {})
        @body = parse_type(data["body"], "WebhookEvent")
        @headers = parse_type(data["headers"], "WebhookHeadersMap")
      end

      def to_h
        {
          "body" => @body,
          "headers" => @headers,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # WebhookDeliveryResponse
    class WebhookDeliveryResponse
      include TypeHelpers
      attr_accessor :code, :headers, :message

      def initialize(data = {})
        @code = parse_integer(data["code"])
        @headers = parse_type(data["headers"], "WebhookHeadersMap")
        @message = data["message"]
      end

      def to_h
        {
          "code" => @code,
          "headers" => @headers,
          "message" => @message,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # WebhookEvent
    class WebhookEvent
      include TypeHelpers
      attr_accessor :copy, :created_at, :creator, :details, :id, :kind, :recording

      def initialize(data = {})
        @copy = parse_type(data["copy"], "WebhookCopy")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
        @details = data["details"]
        @id = parse_integer(data["id"])
        @kind = data["kind"]
        @recording = parse_type(data["recording"], "Recording")
      end

      def to_h
        {
          "copy" => @copy,
          "created_at" => @created_at,
          "creator" => @creator,
          "details" => @details,
          "id" => @id,
          "kind" => @kind,
          "recording" => @recording,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end
  end
end
