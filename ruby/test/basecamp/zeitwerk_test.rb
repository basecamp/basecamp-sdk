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

  # Same hook shape for cards: the merge-safe update sits ahead of the
  # generated class, which keeps the raw PUT as update_verbatim.
  def test_cards_extensions_prepended
    assert_includes Basecamp::Services::CardsService.ancestors, \
                    Basecamp::Services::CardsExtensions
    assert Basecamp::Services::CardsService.ancestors.index(Basecamp::Services::CardsExtensions) <
           Basecamp::Services::CardsService.ancestors.index(Basecamp::Services::CardsService),
           "extensions must be prepended (before the class in the ancestor chain)"
  end

  # And for todolists, where the generated class owns `replace` and the
  # prepended module contributes `update`/`edit`.
  def test_todolists_extensions_prepended
    assert_includes Basecamp::Services::TodolistsService.ancestors, \
                    Basecamp::Services::TodolistsExtensions
    assert Basecamp::Services::TodolistsService.ancestors.index(Basecamp::Services::TodolistsExtensions) <
           Basecamp::Services::TodolistsService.ancestors.index(Basecamp::Services::TodolistsService),
           "extensions must be prepended (before the class in the ancestor chain)"
  end

  # The composite surface only exists if the hook actually ran: `update` and
  # `edit` come from the module, `replace` from the generated class.
  def test_todolists_composite_surface_is_reachable
    assert_equal Basecamp::Services::TodolistsExtensions, \
                 Basecamp::Services::TodolistsService.instance_method(:update).owner
    assert_equal Basecamp::Services::TodolistsExtensions, \
                 Basecamp::Services::TodolistsService.instance_method(:edit).owner
    assert_equal Basecamp::Services::TodolistsService, \
                 Basecamp::Services::TodolistsService.instance_method(:replace).owner
  end

  # And for documents, the same shape as todolists: PUT /documents/{id} is a
  # full replace, so the generated class owns `replace` and the prepended
  # module contributes the merge-safe `update`/`edit`.
  def test_documents_extensions_prepended
    assert_includes Basecamp::Services::DocumentsService.ancestors, \
                    Basecamp::Services::DocumentsExtensions
    assert Basecamp::Services::DocumentsService.ancestors.index(Basecamp::Services::DocumentsExtensions) <
           Basecamp::Services::DocumentsService.ancestors.index(Basecamp::Services::DocumentsService),
           "extensions must be prepended (before the class in the ancestor chain)"
  end

  def test_documents_composite_surface_is_reachable
    assert_equal Basecamp::Services::DocumentsExtensions, \
                 Basecamp::Services::DocumentsService.instance_method(:update).owner
    assert_equal Basecamp::Services::DocumentsExtensions, \
                 Basecamp::Services::DocumentsService.instance_method(:edit).owner
    assert_equal Basecamp::Services::DocumentsService, \
                 Basecamp::Services::DocumentsService.instance_method(:replace).owner
  end

  # And for schedule entries: PUT /schedule_entries/{id} is a full replace, so
  # the generated class owns `replace_entry` and the prepended module
  # contributes the merge-safe `update_entry`/`edit_entry`.
  def test_schedules_extensions_prepended
    assert_includes Basecamp::Services::SchedulesService.ancestors, \
                    Basecamp::Services::SchedulesExtensions
    assert Basecamp::Services::SchedulesService.ancestors.index(Basecamp::Services::SchedulesExtensions) <
           Basecamp::Services::SchedulesService.ancestors.index(Basecamp::Services::SchedulesService),
           "extensions must be prepended (before the class in the ancestor chain)"
  end

  def test_schedules_composite_surface_is_reachable
    assert_equal Basecamp::Services::SchedulesExtensions, \
                 Basecamp::Services::SchedulesService.instance_method(:update_entry).owner
    assert_equal Basecamp::Services::SchedulesExtensions, \
                 Basecamp::Services::SchedulesService.instance_method(:edit_entry).owner
    assert_equal Basecamp::Services::SchedulesService, \
                 Basecamp::Services::SchedulesService.instance_method(:replace_entry).owner
  end

  # And for recordings: the generated class owns the status writes, and the
  # prepended module contributes the `summarize` projection.
  def test_recordings_extensions_prepended
    assert_includes Basecamp::Services::RecordingsService.ancestors, \
                    Basecamp::Services::RecordingsExtensions
    assert Basecamp::Services::RecordingsService.ancestors.index(Basecamp::Services::RecordingsExtensions) <
           Basecamp::Services::RecordingsService.ancestors.index(Basecamp::Services::RecordingsService),
           "extensions must be prepended (before the class in the ancestor chain)"
  end

  def test_recordings_composite_surface_is_reachable
    assert_equal Basecamp::Services::RecordingsExtensions, \
                 Basecamp::Services::RecordingsService.instance_method(:summarize).owner
    assert_equal Basecamp::Services::RecordingsService, \
                 Basecamp::Services::RecordingsService.instance_method(:archive).owner
  end

  # And for comments: the generated class owns `create`, which the prepended
  # `create_with_mentions` composes rather than takes over.
  def test_comments_extensions_prepended
    assert_includes Basecamp::Services::CommentsService.ancestors, \
                    Basecamp::Services::CommentsExtensions
    assert Basecamp::Services::CommentsService.ancestors.index(Basecamp::Services::CommentsExtensions) <
           Basecamp::Services::CommentsService.ancestors.index(Basecamp::Services::CommentsService),
           "extensions must be prepended (before the class in the ancestor chain)"
  end

  def test_comments_composite_surface_is_reachable
    assert_equal Basecamp::Services::CommentsExtensions, \
                 Basecamp::Services::CommentsService.instance_method(:create_with_mentions).owner
    assert_equal Basecamp::Services::CommentsExtensions, \
                 Basecamp::Services::CommentsService.instance_method(:expand_mentions).owner
    assert_equal Basecamp::Services::CommentsService, \
                 Basecamp::Services::CommentsService.instance_method(:create).owner
  end
end
