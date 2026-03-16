# frozen_string_literal: true

module Basecamp
  # Result of downloading file content from a URL.
  DownloadResult = Data.define(:body, :content_type, :content_length, :filename) do
    def initialize(body:, content_type: "", content_length: -1, filename: "download")
      super
    end
  end
end
