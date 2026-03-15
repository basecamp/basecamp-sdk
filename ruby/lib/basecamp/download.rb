# frozen_string_literal: true

require "net/http"
require "uri"

# Download support: result type and URL filename extraction.
module Basecamp
  # Result of downloading file content from a URL.
  DownloadResult = Data.define(:body, :content_type, :content_length, :filename) do
    def initialize(body:, content_type: "", content_length: -1, filename: "download")
      super
    end
  end

  # Extracts a filename from the last path segment of a URL.
  # Falls back to "download" if the URL is unparseable or has no path segments.
  def self.filename_from_url(raw_url)
    uri = URI.parse(raw_url)
    path = uri.path
    return "download" if path.nil? || path.empty? || path == "/" || path.end_with?("/")

    segments = path.split("/").reject(&:empty?)
    return "download" if segments.empty?

    last = segments.last
    return "download" if last.nil? || last.empty? || last == "." || last == "/"

    URI::RFC2396_PARSER.unescape(last)
  rescue URI::InvalidURIError
    "download"
  end
end
