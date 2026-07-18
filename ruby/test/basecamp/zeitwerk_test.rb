require "test_helper"

class ZeitwerkTest < Minitest::Test
  def test_eager_loading
    Zeitwerk::Loader.eager_load_all
  end

  # The on_load hook in basecamp.rb must prepend the hand-written
  # merge-safe update/edit surface onto the generated TodosService.
  def test_todos_extensions_prepended
    assert_includes Basecamp::Services::TodosService.ancestors, \
                    Basecamp::Services::TodosExtensions
    assert Basecamp::Services::TodosService.ancestors.index(Basecamp::Services::TodosExtensions) <
           Basecamp::Services::TodosService.ancestors.index(Basecamp::Services::TodosService),
           "extensions must be prepended (before the class in the ancestor chain)"
  end
end
