package api

import (
	"github.com/emersion/go-imap/v2"
)

func (s *imapServerSession) Namespace() (*imap.NamespaceData, error) {
	return &imap.NamespaceData{
		Personal: []imap.NamespaceDescriptor{
			{
				Prefix: "",
				Delim:  '/',
			},
		},
	}, nil
}
