package app

import (
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

// EmailDeletionSession preserves caller-controlled transaction boundaries.
type EmailDeletionSession interface {
	domain.EmailDeletionRepository
	Begin() error
	Commit() error
	Rollback() error
	Close() error
}

func (s *EmailService) Delete(ctx *context.Context, ids []int, forcedDel bool) error {
	session := s.openDeletion()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := emailDeleteDeleteOne(ctx, session, int64(id), forcedDel); err != nil {
			session.Rollback()
			return err
		}
	}
	return session.Commit()
}

func emailDeleteDeleteOne(ctx *context.Context, repository domain.EmailDeletionRepository, id int64, forcedDel bool) error {
	if !forcedDel {
		return repository.MarkDeleted(ctx.UserID, id)
	}
	if err := repository.RemoveEmailOwner(ctx.UserID, id); err != nil {
		return err
	}
	owners, err := repository.CountEmailOwners(id)
	if err != nil {
		return err
	}
	if owners == 0 {
		return repository.RemoveEmail(id)
	}
	return nil
}

func (s *EmailService) DeleteByUID(ctx *context.Context, ids []int) error {
	session := s.openDeletion()
	defer session.Close()
	for _, id := range ids {
		emailID, found, _ := session.FindEmailByUID(ctx.UserID, id)
		if !found {
			log.WithContext(ctx).Warn("no user email found")
			return nil
		}
		if err := session.RemoveEmailUID(ctx.UserID, id); err != nil {
			session.Rollback()
			return err
		}
		owners, err := session.CountEmailOwners(int64(emailID))
		if err != nil {
			session.Rollback()
			return err
		}
		if owners == 0 {
			if err := session.RemoveEmail(int64(emailID)); err != nil {
				log.WithContext(ctx).Error(err)
			}
		}
	}
	session.Commit()
	return nil
}
