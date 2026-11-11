package api

import (
	"strings"

	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	log "github.com/sirupsen/logrus"
)

func imapMatchGroup(ctx *context.Context, w *imapserver.ListWriter, basePath, pattern string) {
	var groups []*models.Group
	if basePath == "" && pattern == "*" {
		app.Persistence.ListUserGroups(ctx.UserID, &groups)
		//w.WriteList(&imap.ListData{
		//	Attrs:   []imap.MailboxAttr{imap.MailboxAttrNoSelect, imap.MailboxAttrHasChildren},
		//	Delim:   '/',
		//	Mailbox: "[PMail]",
		//})
		w.WriteList(&imap.ListData{
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrHasNoChildren},
			Delim:   '/',
			Mailbox: "INBOX",
		})
		w.WriteList(&imap.ListData{
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrHasNoChildren, imap.MailboxAttrSent},
			Delim:   '/',
			Mailbox: "Sent Messages",
		})
		w.WriteList(&imap.ListData{
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrHasNoChildren, imap.MailboxAttrDrafts},
			Delim:   '/',
			Mailbox: "Drafts",
		})

		w.WriteList(&imap.ListData{
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrHasNoChildren, imap.MailboxAttrTrash},
			Delim:   '/',
			Mailbox: "Deleted Messages",
		})
		w.WriteList(&imap.ListData{
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrHasNoChildren, imap.MailboxAttrJunk},
			Delim:   '/',
			Mailbox: "Junk",
		})
	} else {
		pattern = strings.ReplaceAll(pattern, "/*", "/%")
		app.Persistence.FindGroupsByPathPattern(ctx.UserID, pattern, &groups)

	}
	for _, group := range groups {

		data := &imap.ListData{
			Attrs:   []imap.MailboxAttr{},
			Mailbox: group.Name,
			Delim:   '/',
		}

		if imapHasChildren(ctx, group.ID) {
			data.Attrs = append(data.Attrs, imap.MailboxAttrHasChildren)
		}

		data.Mailbox = imapGetLayerName(ctx, group, true)

		w.WriteList(data)

	}

}

func imapHasChildren(ctx *context.Context, id int) bool {
	var parent []*models.Group
	app.Persistence.FindGroupsByParent(id, &parent)

	return len(parent) > 0
}
func imapGetLayerName(ctx *context.Context, item *models.Group, allPath bool) string {
	if item.ParentId == 0 {
		return item.Name
	}
	var parent models.Group
	_, _ = app.Persistence.FindGroupByID(item.ParentId, &parent)
	if allPath {
		return imapGetLayerName(ctx, &parent, allPath) + "/" + item.Name
	}
	return imapGetLayerName(ctx, &parent, allPath)
}

func (s *imapServerSession) List(w *imapserver.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
	log.WithContext(s.ctx).Debugf("imap server list, ref: %s ,patterns: %s ", ref, patterns)

	if ref == "" && len(patterns) == 0 {
		w.WriteList(&imap.ListData{
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrNoSelect, imap.MailboxAttrHasChildren},
			Delim:   '/',
			Mailbox: "[PMail]",
		})
	}
	for _, pattern := range patterns {
		imapMatchGroup(s.ctx, w, ref, pattern)
	}

	return nil
}
