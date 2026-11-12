package imapapi

import (
	"strings"

	"github.com/Jinnrry/pmail/models"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	log "github.com/sirupsen/logrus"
)

func (s *imapServerSession) matchGroup(w *imapserver.ListWriter, basePath, pattern string) {
	var groups []*models.Group
	if basePath == "" && pattern == "*" {
		groups = s.server.groups.List(s.ctx)
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
		groups = s.server.groups.ListByPathPattern(s.ctx, pattern)
	}
	for _, group := range groups {

		data := &imap.ListData{
			Attrs:   []imap.MailboxAttr{},
			Mailbox: group.Name,
			Delim:   '/',
		}

		if s.server.groups.HasChildren(s.ctx, group.ID) {
			data.Attrs = append(data.Attrs, imap.MailboxAttrHasChildren)
		}

		data.Mailbox = s.server.groups.LayerName(s.ctx, group, true)

		w.WriteList(data)

	}

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
		s.matchGroup(w, ref, pattern)
	}

	return nil
}
