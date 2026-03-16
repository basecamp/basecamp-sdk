require "test_helper"

class ZeitwerkTest < Minitest::Test
  def test_eager_loading
    Zeitwerk::Loader.eager_load_all
  end
end
