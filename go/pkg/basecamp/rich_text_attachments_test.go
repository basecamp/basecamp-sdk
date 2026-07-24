package basecamp

// Wiring proof for the rich text *_attachments coverage sweep (#405): every
// resource that pairs a rich text attribute with a companion attachments array
// must carry that array through its hand-written converter. The table below
// runs one populated generated fixture through each converter and asserts the
// relevant array(s) convert, exercising the shared
// richTextAttachmentsFromGenerated helper for all 15 converter structures (13
// concrete + the two polymorphic projections, each of which carries both a
// content and a description array). Gauge and GaugeNeedle decode directly (no
// converter) and are covered in gauges_test.go.

import (
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// richTextAttachmentsFixture returns a two-element generated attachment slice
// exercising both dimension forms: an image with a pixel width and a non-image
// blob with nil (JSON null) dimensions.
func richTextAttachmentsFixture() []generated.RichTextAttachment {
	w := types.FlexInt(1024)
	h := types.FlexInt(768)
	return []generated.RichTextAttachment{
		{
			Id:           987,
			Sgid:         "BAh7img",
			Filename:     "diagram.png",
			ContentType:  "image/png",
			ByteSize:     20480,
			DownloadUrl:  "https://example.com/download/diagram.png",
			Width:        &w,
			Height:       &h,
			Previewable:  true,
			PreviewUrl:   "https://example.com/preview/diagram.png",
			ThumbnailUrl: "https://example.com/thumb/diagram.png",
		},
		{
			Id:           988,
			Sgid:         "BAh7pdf",
			Filename:     "spec.pdf",
			ContentType:  "application/pdf",
			ByteSize:     51200,
			DownloadUrl:  "https://example.com/download/spec.pdf",
			Width:        nil,
			Height:       nil,
			Previewable:  false,
			PreviewUrl:   "https://example.com/preview/spec.pdf",
			ThumbnailUrl: "https://example.com/thumb/spec.pdf",
		},
	}
}

// assertRichTextAttachmentsConverted verifies the shared fixture converted to
// the public type with the always-emitted fields carried through and both
// dimension forms narrowed faithfully (present -> *int32, null -> nil).
func assertRichTextAttachmentsConverted(t *testing.T, got []RichTextAttachment) {
	t.Helper()
	if len(got) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(got))
	}
	img := got[0]
	if img.ID != 987 || img.SGID != "BAh7img" || img.Filename != "diagram.png" || img.ContentType != "image/png" {
		t.Errorf("image attachment fields not propagated: %+v", img)
	}
	if img.ByteSize != 20480 || img.DownloadURL != "https://example.com/download/diagram.png" {
		t.Errorf("image attachment byte_size/download_url not propagated: %+v", img)
	}
	if img.Width == nil || *img.Width != 1024 {
		t.Errorf("image Width: got %v, want 1024", img.Width)
	}
	if img.Height == nil || *img.Height != 768 {
		t.Errorf("image Height: got %v, want 768", img.Height)
	}
	if !img.Previewable {
		t.Error("image Previewable: got false, want true")
	}
	blob := got[1]
	if blob.ID != 988 || blob.ContentType != "application/pdf" {
		t.Errorf("blob attachment fields not propagated: %+v", blob)
	}
	if blob.Width != nil || blob.Height != nil {
		t.Errorf("blob dimensions: got width=%v height=%v, want nil/nil", blob.Width, blob.Height)
	}
}

func TestRichTextAttachments_ConverterPropagation(t *testing.T) {
	fix := richTextAttachmentsFixture()
	cases := []struct {
		name string
		got  []RichTextAttachment
	}{
		{"Todolist", todolistFromGenerated(generated.Todolist{DescriptionAttachments: fix}).DescriptionAttachments},
		{"Comment", commentFromGenerated(generated.Comment{ContentAttachments: fix}).ContentAttachments},
		{"Message", messageFromGenerated(generated.Message{ContentAttachments: fix}).ContentAttachments},
		{"Document", documentFromGenerated(generated.Document{ContentAttachments: fix}).ContentAttachments},
		{"Upload", uploadFromGenerated(generated.Upload{DescriptionAttachments: fix}).DescriptionAttachments},
		{"ScheduleEntry", scheduleEntryFromGenerated(generated.ScheduleEntry{DescriptionAttachments: fix}).DescriptionAttachments},
		{"Forward", forwardFromGenerated(generated.Forward{ContentAttachments: fix}).ContentAttachments},
		{"ForwardReply", forwardReplyFromGenerated(generated.ForwardReply{ContentAttachments: fix}).ContentAttachments},
		{"Card", cardFromGenerated(generated.Card{DescriptionAttachments: fix}).DescriptionAttachments},
		{"ClientApproval", clientApprovalFromGenerated(generated.ClientApproval{ContentAttachments: fix}).ContentAttachments},
		{"ClientCorrespondence", clientCorrespondenceFromGenerated(generated.ClientCorrespondence{ContentAttachments: fix}).ContentAttachments},
		{"ClientReply", clientReplyFromGenerated(generated.ClientReply{ContentAttachments: fix}).ContentAttachments},
		{"QuestionAnswer", questionAnswerFromGenerated(generated.QuestionAnswer{ContentAttachments: fix}).ContentAttachments},
		{"SearchResult.content", searchResultFromGenerated(generated.SearchResult{ContentAttachments: fix}).ContentAttachments},
		{"SearchResult.description", searchResultFromGenerated(generated.SearchResult{DescriptionAttachments: fix}).DescriptionAttachments},
		{"Recording.content", recordingFromGenerated(generated.Recording{ContentAttachments: fix}).ContentAttachments},
		{"Recording.description", recordingFromGenerated(generated.Recording{DescriptionAttachments: fix}).DescriptionAttachments},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertRichTextAttachmentsConverted(t, tc.got)
		})
	}
}

// TestRichTextAttachments_NilVsEmpty pins the nil-vs-empty contract the shared
// helper preserves: a server-sent [] becomes a non-nil zero-length slice, an
// absent property stays nil. Representative of every converter structure since
// they all route through richTextAttachmentsFromGenerated.
func TestRichTextAttachments_NilVsEmpty(t *testing.T) {
	// Present but empty -> non-nil zero-length slice.
	c := commentFromGenerated(generated.Comment{ContentAttachments: []generated.RichTextAttachment{}})
	if c.ContentAttachments == nil {
		t.Error("expected non-nil ContentAttachments for server-sent []")
	}
	if len(c.ContentAttachments) != 0 {
		t.Errorf("expected 0 attachments, got %d", len(c.ContentAttachments))
	}

	// Absent -> nil.
	c = commentFromGenerated(generated.Comment{})
	if c.ContentAttachments != nil {
		t.Errorf("expected nil ContentAttachments for absent property, got %v", c.ContentAttachments)
	}
}
