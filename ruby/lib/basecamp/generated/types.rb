# frozen_string_literal: true

# Auto-generated from OpenAPI spec. Do not edit manually.
# Generated: 2026-02-05T07:44:31Z

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
      attr_accessor :id, :title, :type, :url, :app_url, :bucket, :parent, :due_on, :starts_on, :assignees

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @title = data["title"]
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @parent = parse_type(data["parent"], "TodoParent")
        @due_on = data["due_on"]
        @starts_on = data["starts_on"]
        @assignees = parse_array(data["assignees"], "Person")
      end

      def to_h
        {
          "id" => @id,
          "title" => @title,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bucket" => @bucket,
          "parent" => @parent,
          "due_on" => @due_on,
          "starts_on" => @starts_on,
          "assignees" => @assignees,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Campfire
    class Campfire
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :position, :bucket, :creator, :topic, :lines_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @position = parse_integer(data["position"])
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @topic = data["topic"]
        @lines_url = data["lines_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "position" => @position,
          "bucket" => @bucket,
          "creator" => @creator,
          "topic" => @topic,
          "lines_url" => @lines_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # CampfireLine
    class CampfireLine
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :content, :parent, :bucket, :creator

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @content = data["content"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "content" => @content,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Card
    class Card
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :position, :content, :description, :due_on, :completed, :completed_at, :comments_count, :comments_url, :completion_url, :parent, :bucket, :creator, :completer, :assignees, :completion_subscribers, :steps

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @position = parse_integer(data["position"])
        @content = data["content"]
        @description = data["description"]
        @due_on = data["due_on"]
        @completed = parse_boolean(data["completed"])
        @completed_at = parse_datetime(data["completed_at"])
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @completion_url = data["completion_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @completer = parse_type(data["completer"], "Person")
        @assignees = parse_array(data["assignees"], "Person")
        @completion_subscribers = parse_array(data["completion_subscribers"], "Person")
        @steps = parse_array(data["steps"], "CardStep")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "position" => @position,
          "content" => @content,
          "description" => @description,
          "due_on" => @due_on,
          "completed" => @completed,
          "completed_at" => @completed_at,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "completion_url" => @completion_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "completer" => @completer,
          "assignees" => @assignees,
          "completion_subscribers" => @completion_subscribers,
          "steps" => @steps,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # CardColumn
    class CardColumn
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :position, :color, :description, :cards_count, :comments_count, :cards_url, :parent, :bucket, :creator, :subscribers

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @position = parse_integer(data["position"])
        @color = data["color"]
        @description = data["description"]
        @cards_count = parse_integer(data["cards_count"])
        @comments_count = parse_integer(data["comments_count"])
        @cards_url = data["cards_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @subscribers = parse_array(data["subscribers"], "Person")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "position" => @position,
          "color" => @color,
          "description" => @description,
          "cards_count" => @cards_count,
          "comments_count" => @comments_count,
          "cards_url" => @cards_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
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
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :position, :due_on, :completed, :completed_at, :parent, :bucket, :creator, :completer, :assignees, :completion_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @position = parse_integer(data["position"])
        @due_on = data["due_on"]
        @completed = parse_boolean(data["completed"])
        @completed_at = parse_datetime(data["completed_at"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @completer = parse_type(data["completer"], "Person")
        @assignees = parse_array(data["assignees"], "Person")
        @completion_url = data["completion_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "position" => @position,
          "due_on" => @due_on,
          "completed" => @completed,
          "completed_at" => @completed_at,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "completer" => @completer,
          "assignees" => @assignees,
          "completion_url" => @completion_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # CardTable
    class CardTable
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :bucket, :creator, :subscribers, :lists

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @subscribers = parse_array(data["subscribers"], "Person")
        @lists = parse_array(data["lists"], "CardColumn")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "bucket" => @bucket,
          "creator" => @creator,
          "subscribers" => @subscribers,
          "lists" => @lists,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Chatbot
    class Chatbot
      include TypeHelpers
      attr_accessor :id, :created_at, :updated_at, :service_name, :command_url, :url, :app_url, :lines_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @service_name = data["service_name"]
        @command_url = data["command_url"]
        @url = data["url"]
        @app_url = data["app_url"]
        @lines_url = data["lines_url"]
      end

      def to_h
        {
          "id" => @id,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "service_name" => @service_name,
          "command_url" => @command_url,
          "url" => @url,
          "app_url" => @app_url,
          "lines_url" => @lines_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ClientApproval
    class ClientApproval
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :parent, :bucket, :creator, :content, :subject, :due_on, :replies_count, :replies_url, :approval_status, :approver, :responses

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @creator = parse_type(data["creator"], "Person")
        @content = data["content"]
        @subject = data["subject"]
        @due_on = data["due_on"]
        @replies_count = parse_integer(data["replies_count"])
        @replies_url = data["replies_url"]
        @approval_status = data["approval_status"]
        @approver = parse_type(data["approver"], "Person")
        @responses = parse_array(data["responses"], "ClientApprovalResponse")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "content" => @content,
          "subject" => @subject,
          "due_on" => @due_on,
          "replies_count" => @replies_count,
          "replies_url" => @replies_url,
          "approval_status" => @approval_status,
          "approver" => @approver,
          "responses" => @responses,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ClientApprovalResponse
    class ClientApprovalResponse
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :app_url, :bookmark_url, :parent, :bucket, :creator, :content, :approved

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @creator = parse_type(data["creator"], "Person")
        @content = data["content"]
        @approved = parse_boolean(data["approved"])
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "content" => @content,
          "approved" => @approved,
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
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :parent, :bucket, :creator, :content, :subject, :replies_count, :replies_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @creator = parse_type(data["creator"], "Person")
        @content = data["content"]
        @subject = data["subject"]
        @replies_count = parse_integer(data["replies_count"])
        @replies_url = data["replies_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "content" => @content,
          "subject" => @subject,
          "replies_count" => @replies_count,
          "replies_url" => @replies_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ClientReply
    class ClientReply
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :parent, :bucket, :creator, :content

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @creator = parse_type(data["creator"], "Person")
        @content = data["content"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "content" => @content,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ClientSide
    class ClientSide
      include TypeHelpers
      attr_accessor :url, :app_url

      def initialize(data = {})
        @url = data["url"]
        @app_url = data["app_url"]
      end

      def to_h
        {
          "url" => @url,
          "app_url" => @app_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Comment
    class Comment
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :parent, :bucket, :creator, :content

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @content = data["content"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "content" => @content,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # CreatePersonRequest
    class CreatePersonRequest
      include TypeHelpers
      attr_accessor :name, :email_address, :title, :company_name

      def initialize(data = {})
        @name = data["name"]
        @email_address = data["email_address"]
        @title = data["title"]
        @company_name = data["company_name"]
      end

      def to_h
        {
          "name" => @name,
          "email_address" => @email_address,
          "title" => @title,
          "company_name" => @company_name,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # DockItem
    class DockItem
      include TypeHelpers
      attr_accessor :id, :title, :name, :enabled, :position, :url, :app_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @title = data["title"]
        @name = data["name"]
        @enabled = parse_boolean(data["enabled"])
        @position = parse_integer(data["position"])
        @url = data["url"]
        @app_url = data["app_url"]
      end

      def to_h
        {
          "id" => @id,
          "title" => @title,
          "name" => @name,
          "enabled" => @enabled,
          "position" => @position,
          "url" => @url,
          "app_url" => @app_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Document
    class Document
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :comments_count, :comments_url, :position, :parent, :bucket, :creator, :content

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @position = parse_integer(data["position"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @content = data["content"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "position" => @position,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "content" => @content,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Event
    class Event
      include TypeHelpers
      attr_accessor :id, :recording_id, :action, :details, :created_at, :creator

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @recording_id = parse_integer(data["recording_id"])
        @action = data["action"]
        @details = parse_type(data["details"], "EventDetails")
        @created_at = parse_datetime(data["created_at"])
        @creator = parse_type(data["creator"], "Person")
      end

      def to_h
        {
          "id" => @id,
          "recording_id" => @recording_id,
          "action" => @action,
          "details" => @details,
          "created_at" => @created_at,
          "creator" => @creator,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # EventDetails
    class EventDetails
      include TypeHelpers
      attr_accessor :added_person_ids, :removed_person_ids, :notified_recipient_ids

      def initialize(data = {})
        @added_person_ids = data["added_person_ids"]
        @removed_person_ids = data["removed_person_ids"]
        @notified_recipient_ids = data["notified_recipient_ids"]
      end

      def to_h
        {
          "added_person_ids" => @added_person_ids,
          "removed_person_ids" => @removed_person_ids,
          "notified_recipient_ids" => @notified_recipient_ids,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Forward
    class Forward
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :parent, :bucket, :creator, :content, :subject, :from, :replies_count, :replies_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @content = data["content"]
        @subject = data["subject"]
        @from = data["from"]
        @replies_count = parse_integer(data["replies_count"])
        @replies_url = data["replies_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "content" => @content,
          "subject" => @subject,
          "from" => @from,
          "replies_count" => @replies_count,
          "replies_url" => @replies_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ForwardReply
    class ForwardReply
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :parent, :bucket, :creator, :content

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @content = data["content"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "content" => @content,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Inbox
    class Inbox
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :position, :bucket, :creator, :forwards_count, :forwards_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @position = parse_integer(data["position"])
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @forwards_count = parse_integer(data["forwards_count"])
        @forwards_url = data["forwards_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "position" => @position,
          "bucket" => @bucket,
          "creator" => @creator,
          "forwards_count" => @forwards_count,
          "forwards_url" => @forwards_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Message
    class Message
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :comments_count, :comments_url, :parent, :bucket, :creator, :subject, :content, :category

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @subject = data["subject"]
        @content = data["content"]
        @category = parse_type(data["category"], "MessageType")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "subject" => @subject,
          "content" => @content,
          "category" => @category,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # MessageBoard
    class MessageBoard
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :position, :bucket, :creator, :messages_count, :messages_url, :app_messages_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @position = parse_integer(data["position"])
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @messages_count = parse_integer(data["messages_count"])
        @messages_url = data["messages_url"]
        @app_messages_url = data["app_messages_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "position" => @position,
          "bucket" => @bucket,
          "creator" => @creator,
          "messages_count" => @messages_count,
          "messages_url" => @messages_url,
          "app_messages_url" => @app_messages_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # MessageType
    class MessageType
      include TypeHelpers
      attr_accessor :id, :name, :icon, :created_at, :updated_at

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @name = data["name"]
        @icon = data["icon"]
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
      end

      def to_h
        {
          "id" => @id,
          "name" => @name,
          "icon" => @icon,
          "created_at" => @created_at,
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
      attr_accessor :id, :attachable_sgid, :name, :email_address, :personable_type, :title, :bio, :location, :created_at, :updated_at, :admin, :owner, :client, :employee, :time_zone, :avatar_url, :company, :can_manage_projects, :can_manage_people

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @attachable_sgid = data["attachable_sgid"]
        @name = data["name"]
        @email_address = data["email_address"]
        @personable_type = data["personable_type"]
        @title = data["title"]
        @bio = data["bio"]
        @location = data["location"]
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @admin = parse_boolean(data["admin"])
        @owner = parse_boolean(data["owner"])
        @client = parse_boolean(data["client"])
        @employee = parse_boolean(data["employee"])
        @time_zone = data["time_zone"]
        @avatar_url = data["avatar_url"]
        @company = parse_type(data["company"], "PersonCompany")
        @can_manage_projects = parse_boolean(data["can_manage_projects"])
        @can_manage_people = parse_boolean(data["can_manage_people"])
      end

      def to_h
        {
          "id" => @id,
          "attachable_sgid" => @attachable_sgid,
          "name" => @name,
          "email_address" => @email_address,
          "personable_type" => @personable_type,
          "title" => @title,
          "bio" => @bio,
          "location" => @location,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "admin" => @admin,
          "owner" => @owner,
          "client" => @client,
          "employee" => @employee,
          "time_zone" => @time_zone,
          "avatar_url" => @avatar_url,
          "company" => @company,
          "can_manage_projects" => @can_manage_projects,
          "can_manage_people" => @can_manage_people,
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
      attr_accessor :id, :status, :created_at, :updated_at, :name, :description, :purpose, :clients_enabled, :bookmark_url, :url, :app_url, :dock, :bookmarked, :client_company, :clientside

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @name = data["name"]
        @description = data["description"]
        @purpose = data["purpose"]
        @clients_enabled = parse_boolean(data["clients_enabled"])
        @bookmark_url = data["bookmark_url"]
        @url = data["url"]
        @app_url = data["app_url"]
        @dock = parse_array(data["dock"], "DockItem")
        @bookmarked = parse_boolean(data["bookmarked"])
        @client_company = parse_type(data["client_company"], "ClientCompany")
        @clientside = parse_type(data["clientside"], "ClientSide")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "name" => @name,
          "description" => @description,
          "purpose" => @purpose,
          "clients_enabled" => @clients_enabled,
          "bookmark_url" => @bookmark_url,
          "url" => @url,
          "app_url" => @app_url,
          "dock" => @dock,
          "bookmarked" => @bookmarked,
          "client_company" => @client_company,
          "clientside" => @clientside,
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
      attr_accessor :id, :status, :url, :project

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @url = data["url"]
        @project = parse_type(data["project"], "Project")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "url" => @url,
          "project" => @project,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Question
    class Question
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :parent, :bucket, :creator, :paused, :schedule, :answers_count, :answers_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @creator = parse_type(data["creator"], "Person")
        @paused = parse_boolean(data["paused"])
        @schedule = parse_type(data["schedule"], "QuestionSchedule")
        @answers_count = parse_integer(data["answers_count"])
        @answers_url = data["answers_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "paused" => @paused,
          "schedule" => @schedule,
          "answers_count" => @answers_count,
          "answers_url" => @answers_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # QuestionAnswer
    class QuestionAnswer
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :comments_count, :comments_url, :content, :group_on, :parent, :bucket, :creator

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @content = data["content"]
        @group_on = data["group_on"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @creator = parse_type(data["creator"], "Person")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "content" => @content,
          "group_on" => @group_on,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
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
      attr_accessor :reminder_id, :remind_at, :group_on, :question

      def initialize(data = {})
        @reminder_id = parse_integer(data["reminder_id"])
        @remind_at = parse_datetime(data["remind_at"])
        @group_on = data["group_on"]
        @question = parse_type(data["question"], "Question")
      end

      def to_h
        {
          "reminder_id" => @reminder_id,
          "remind_at" => @remind_at,
          "group_on" => @group_on,
          "question" => @question,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # QuestionSchedule
    class QuestionSchedule
      include TypeHelpers
      attr_accessor :frequency, :days, :hour, :minute, :week_instance, :week_interval, :month_interval, :start_date, :end_date

      def initialize(data = {})
        @frequency = data["frequency"]
        @days = data["days"]
        @hour = parse_integer(data["hour"])
        @minute = parse_integer(data["minute"])
        @week_instance = parse_integer(data["week_instance"])
        @week_interval = parse_integer(data["week_interval"])
        @month_interval = parse_integer(data["month_interval"])
        @start_date = data["start_date"]
        @end_date = data["end_date"]
      end

      def to_h
        {
          "frequency" => @frequency,
          "days" => @days,
          "hour" => @hour,
          "minute" => @minute,
          "week_instance" => @week_instance,
          "week_interval" => @week_interval,
          "month_interval" => @month_interval,
          "start_date" => @start_date,
          "end_date" => @end_date,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Questionnaire
    class Questionnaire
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :questions_url, :questions_count, :name, :bucket, :creator

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @questions_url = data["questions_url"]
        @questions_count = parse_integer(data["questions_count"])
        @name = data["name"]
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @creator = parse_type(data["creator"], "Person")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "questions_url" => @questions_url,
          "questions_count" => @questions_count,
          "name" => @name,
          "bucket" => @bucket,
          "creator" => @creator,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Recording
    class Recording
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :parent, :bucket, :creator

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @creator = parse_type(data["creator"], "Person")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
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
      attr_accessor :id, :title, :type, :url, :app_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @title = data["title"]
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
      end

      def to_h
        {
          "id" => @id,
          "title" => @title,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Schedule
    class Schedule
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :position, :bucket, :creator, :include_due_assignments, :entries_count, :entries_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @position = parse_integer(data["position"])
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @include_due_assignments = parse_boolean(data["include_due_assignments"])
        @entries_count = parse_integer(data["entries_count"])
        @entries_url = data["entries_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "position" => @position,
          "bucket" => @bucket,
          "creator" => @creator,
          "include_due_assignments" => @include_due_assignments,
          "entries_count" => @entries_count,
          "entries_url" => @entries_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ScheduleAttributes
    class ScheduleAttributes
      include TypeHelpers
      attr_accessor :start_date, :end_date

      def initialize(data = {})
        @start_date = data["start_date"]
        @end_date = data["end_date"]
      end

      def to_h
        {
          "start_date" => @start_date,
          "end_date" => @end_date,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # ScheduleEntry
    class ScheduleEntry
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :comments_count, :comments_url, :parent, :bucket, :creator, :summary, :description, :all_day, :starts_at, :ends_at, :participants

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @summary = data["summary"]
        @description = data["description"]
        @all_day = parse_boolean(data["all_day"])
        @starts_at = parse_datetime(data["starts_at"])
        @ends_at = parse_datetime(data["ends_at"])
        @participants = parse_array(data["participants"], "Person")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "summary" => @summary,
          "description" => @description,
          "all_day" => @all_day,
          "starts_at" => @starts_at,
          "ends_at" => @ends_at,
          "participants" => @participants,
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
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :parent, :bucket, :creator, :content, :description, :subject

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "RecordingBucket")
        @creator = parse_type(data["creator"], "Person")
        @content = data["content"]
        @description = data["description"]
        @subject = data["subject"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "content" => @content,
          "description" => @description,
          "subject" => @subject,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Subscription
    class Subscription
      include TypeHelpers
      attr_accessor :subscribed, :count, :url, :subscribers

      def initialize(data = {})
        @subscribed = parse_boolean(data["subscribed"])
        @count = parse_integer(data["count"])
        @url = data["url"]
        @subscribers = parse_array(data["subscribers"], "Person")
      end

      def to_h
        {
          "subscribed" => @subscribed,
          "count" => @count,
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
      attr_accessor :id, :status, :created_at, :updated_at, :name, :description, :url, :app_url, :dock

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @name = data["name"]
        @description = data["description"]
        @url = data["url"]
        @app_url = data["app_url"]
        @dock = parse_array(data["dock"], "DockItem")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "name" => @name,
          "description" => @description,
          "url" => @url,
          "app_url" => @app_url,
          "dock" => @dock,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # TimelineEvent
    class TimelineEvent
      include TypeHelpers
      attr_accessor :id, :created_at, :kind, :parent_recording_id, :url, :app_url, :creator, :action, :target, :title, :summary_excerpt, :bucket

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @created_at = parse_datetime(data["created_at"])
        @kind = data["kind"]
        @parent_recording_id = parse_integer(data["parent_recording_id"])
        @url = data["url"]
        @app_url = data["app_url"]
        @creator = parse_type(data["creator"], "Person")
        @action = data["action"]
        @target = data["target"]
        @title = data["title"]
        @summary_excerpt = data["summary_excerpt"]
        @bucket = parse_type(data["bucket"], "TodoBucket")
      end

      def to_h
        {
          "id" => @id,
          "created_at" => @created_at,
          "kind" => @kind,
          "parent_recording_id" => @parent_recording_id,
          "url" => @url,
          "app_url" => @app_url,
          "creator" => @creator,
          "action" => @action,
          "target" => @target,
          "title" => @title,
          "summary_excerpt" => @summary_excerpt,
          "bucket" => @bucket,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # TimesheetEntry
    class TimesheetEntry
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :parent, :bucket, :creator, :date, :description, :hours, :person

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @date = data["date"]
        @description = data["description"]
        @hours = data["hours"]
        @person = parse_type(data["person"], "Person")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
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
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :comments_count, :comments_url, :position, :parent, :bucket, :creator, :description, :completed, :content, :starts_on, :due_on, :assignees, :completion_subscribers, :completion_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @position = parse_integer(data["position"])
        @parent = parse_type(data["parent"], "TodoParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @description = data["description"]
        @completed = parse_boolean(data["completed"])
        @content = data["content"]
        @starts_on = data["starts_on"]
        @due_on = data["due_on"]
        @assignees = parse_array(data["assignees"], "Person")
        @completion_subscribers = parse_array(data["completion_subscribers"], "Person")
        @completion_url = data["completion_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "position" => @position,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "description" => @description,
          "completed" => @completed,
          "content" => @content,
          "starts_on" => @starts_on,
          "due_on" => @due_on,
          "assignees" => @assignees,
          "completion_subscribers" => @completion_subscribers,
          "completion_url" => @completion_url,
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
      attr_accessor :id, :title, :type, :url, :app_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @title = data["title"]
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
      end

      def to_h
        {
          "id" => @id,
          "title" => @title,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Todolist
    class Todolist
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :comments_count, :comments_url, :position, :parent, :bucket, :creator, :description, :completed, :completed_ratio, :name, :todos_url, :groups_url, :app_todos_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @position = parse_integer(data["position"])
        @parent = parse_type(data["parent"], "TodoParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @description = data["description"]
        @completed = parse_boolean(data["completed"])
        @completed_ratio = data["completed_ratio"]
        @name = data["name"]
        @todos_url = data["todos_url"]
        @groups_url = data["groups_url"]
        @app_todos_url = data["app_todos_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "position" => @position,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "description" => @description,
          "completed" => @completed,
          "completed_ratio" => @completed_ratio,
          "name" => @name,
          "todos_url" => @todos_url,
          "groups_url" => @groups_url,
          "app_todos_url" => @app_todos_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # TodolistGroup
    class TodolistGroup
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :comments_count, :comments_url, :position, :parent, :bucket, :creator, :name, :completed, :completed_ratio, :todos_url, :app_todos_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @position = parse_integer(data["position"])
        @parent = parse_type(data["parent"], "TodoParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @name = data["name"]
        @completed = parse_boolean(data["completed"])
        @completed_ratio = data["completed_ratio"]
        @todos_url = data["todos_url"]
        @app_todos_url = data["app_todos_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "position" => @position,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "name" => @name,
          "completed" => @completed,
          "completed_ratio" => @completed_ratio,
          "todos_url" => @todos_url,
          "app_todos_url" => @app_todos_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Todoset
    class Todoset
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :position, :bucket, :creator, :name, :todolists_count, :todolists_url, :completed_ratio, :completed, :completed_count, :on_schedule_count, :over_schedule_count, :app_todolists_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @position = parse_integer(data["position"])
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @name = data["name"]
        @todolists_count = parse_integer(data["todolists_count"])
        @todolists_url = data["todolists_url"]
        @completed_ratio = data["completed_ratio"]
        @completed = parse_boolean(data["completed"])
        @completed_count = parse_integer(data["completed_count"])
        @on_schedule_count = parse_integer(data["on_schedule_count"])
        @over_schedule_count = parse_integer(data["over_schedule_count"])
        @app_todolists_url = data["app_todolists_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "position" => @position,
          "bucket" => @bucket,
          "creator" => @creator,
          "name" => @name,
          "todolists_count" => @todolists_count,
          "todolists_url" => @todolists_url,
          "completed_ratio" => @completed_ratio,
          "completed" => @completed,
          "completed_count" => @completed_count,
          "on_schedule_count" => @on_schedule_count,
          "over_schedule_count" => @over_schedule_count,
          "app_todolists_url" => @app_todolists_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Tool
    class Tool
      include TypeHelpers
      attr_accessor :id, :status, :created_at, :updated_at, :title, :name, :enabled, :position, :url, :app_url, :bucket

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @name = data["name"]
        @enabled = parse_boolean(data["enabled"])
        @position = parse_integer(data["position"])
        @url = data["url"]
        @app_url = data["app_url"]
        @bucket = parse_type(data["bucket"], "RecordingBucket")
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "name" => @name,
          "enabled" => @enabled,
          "position" => @position,
          "url" => @url,
          "app_url" => @app_url,
          "bucket" => @bucket,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Upload
    class Upload
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :subscription_url, :comments_count, :comments_url, :position, :parent, :bucket, :creator, :description, :content_type, :byte_size, :width, :height, :download_url, :filename

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @subscription_url = data["subscription_url"]
        @comments_count = parse_integer(data["comments_count"])
        @comments_url = data["comments_url"]
        @position = parse_integer(data["position"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @description = data["description"]
        @content_type = data["content_type"]
        @byte_size = parse_integer(data["byte_size"])
        @width = parse_integer(data["width"])
        @height = parse_integer(data["height"])
        @download_url = data["download_url"]
        @filename = data["filename"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "subscription_url" => @subscription_url,
          "comments_count" => @comments_count,
          "comments_url" => @comments_url,
          "position" => @position,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "description" => @description,
          "content_type" => @content_type,
          "byte_size" => @byte_size,
          "width" => @width,
          "height" => @height,
          "download_url" => @download_url,
          "filename" => @filename,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end

    # Vault
    class Vault
      include TypeHelpers
      attr_accessor :id, :status, :visible_to_clients, :created_at, :updated_at, :title, :inherits_status, :type, :url, :app_url, :bookmark_url, :position, :parent, :bucket, :creator, :documents_count, :documents_url, :uploads_count, :uploads_url, :vaults_count, :vaults_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @status = data["status"]
        @visible_to_clients = parse_boolean(data["visible_to_clients"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @title = data["title"]
        @inherits_status = parse_boolean(data["inherits_status"])
        @type = data["type"]
        @url = data["url"]
        @app_url = data["app_url"]
        @bookmark_url = data["bookmark_url"]
        @position = parse_integer(data["position"])
        @parent = parse_type(data["parent"], "RecordingParent")
        @bucket = parse_type(data["bucket"], "TodoBucket")
        @creator = parse_type(data["creator"], "Person")
        @documents_count = parse_integer(data["documents_count"])
        @documents_url = data["documents_url"]
        @uploads_count = parse_integer(data["uploads_count"])
        @uploads_url = data["uploads_url"]
        @vaults_count = parse_integer(data["vaults_count"])
        @vaults_url = data["vaults_url"]
      end

      def to_h
        {
          "id" => @id,
          "status" => @status,
          "visible_to_clients" => @visible_to_clients,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "title" => @title,
          "inherits_status" => @inherits_status,
          "type" => @type,
          "url" => @url,
          "app_url" => @app_url,
          "bookmark_url" => @bookmark_url,
          "position" => @position,
          "parent" => @parent,
          "bucket" => @bucket,
          "creator" => @creator,
          "documents_count" => @documents_count,
          "documents_url" => @documents_url,
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
      attr_accessor :id, :active, :created_at, :updated_at, :payload_url, :types, :url, :app_url

      def initialize(data = {})
        @id = parse_integer(data["id"])
        @active = parse_boolean(data["active"])
        @created_at = parse_datetime(data["created_at"])
        @updated_at = parse_datetime(data["updated_at"])
        @payload_url = data["payload_url"]
        @types = data["types"]
        @url = data["url"]
        @app_url = data["app_url"]
      end

      def to_h
        {
          "id" => @id,
          "active" => @active,
          "created_at" => @created_at,
          "updated_at" => @updated_at,
          "payload_url" => @payload_url,
          "types" => @types,
          "url" => @url,
          "app_url" => @app_url,
        }.compact
      end

      def to_json(*args)
        to_h.to_json(*args)
      end
    end
  end
end
