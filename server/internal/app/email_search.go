package app

import (
	"time"

	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/emersion/go-imap/v2"
)

// IMAPSearchCriteria represents the search criteria for IMAP SEARCH command
type EmailListIMAPSearchCriteria struct {
	// UID and sequence number sets
	UIDs    []imap.UIDSet
	SeqNums []imap.SeqSet

	// Date filters (only date is used, time and timezone are ignored)
	Since      time.Time // Internal date since
	Before     time.Time // Internal date before
	SentSince  time.Time // Sent date since
	SentBefore time.Time // Sent date before

	// Header field searches
	Header []EmailListHeaderField // Key-Value pairs for header search

	// Content search
	Body []string // Search in body
	Text []string // Search in headers + body

	// Flag filters
	Flag    []imap.Flag // Messages with these flags
	NotFlag []imap.Flag // Messages without these flags

	// Size filters
	Larger  int64 // Messages larger than this (bytes)
	Smaller int64 // Messages smaller than this (bytes)

	// Logical combinations
	Not []EmailListIMAPSearchCriteria
	Or  [][2]EmailListIMAPSearchCriteria
}

// HeaderField represents a header field search criterion
type EmailListHeaderField struct {
	Key   string
	Value string
}

// IMAPSearchResult represents a search result item
type EmailListIMAPSearchResult struct {
	UID          int  // user_email.id as UID
	SeqNum       int  // sequence number
	EmailID      int  // email.id
	IsRead       int8 // read status
	SerialNumber int  // serial number
}

// SearchEmails performs IMAP search based on criteria
func (s *EmailQueryService) Search(ctx *context.Context, groupName string, criteria *imap.SearchCriteria) ([]*response.UserEmailUIDData, error) {
	// First get the base list for the mailbox
	baseList := s.ListByUID(ctx, groupName, 0, 0, nil)
	if len(baseList) == 0 {
		return baseList, nil
	}

	// If no criteria specified, return all
	if criteria == nil || emailListIsEmptyCriteria(criteria) {
		return baseList, nil
	}

	// Build UID to sequence number mapping
	uidToSeq := make(map[int]int)
	seqToUID := make(map[int]int)
	for _, item := range baseList {
		uidToSeq[item.ID] = item.SerialNumber
		seqToUID[item.SerialNumber] = item.ID
	}

	// Filter by UID sets
	if len(criteria.UID) > 0 {
		baseList = emailListFilterByUIDSets(baseList, criteria.UID)
	}

	// Filter by sequence number sets
	if len(criteria.SeqNum) > 0 {
		baseList = emailListFilterBySeqNumSets(baseList, criteria.SeqNum)
	}

	// For more complex filters, we need to fetch email data
	if emailListNeedsEmailData(criteria) {
		baseList = s.filterWithEmailData(ctx, baseList, criteria)
	}

	// Filter by flags (is_read status in this implementation)
	if len(criteria.Flag) > 0 || len(criteria.NotFlag) > 0 {
		baseList = emailListFilterByFlags(baseList, criteria.Flag, criteria.NotFlag)
	}

	// Handle NOT criteria
	if len(criteria.Not) > 0 {
		for _, notCriteria := range criteria.Not {
			baseList = s.applyNotCriteria(ctx, groupName, baseList, &notCriteria)
		}
	}

	// Handle OR criteria
	if len(criteria.Or) > 0 {
		baseList = s.applyOrCriteria(ctx, groupName, baseList, criteria.Or)
	}

	return baseList, nil
}
