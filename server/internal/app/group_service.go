package app

import (
	errors2 "errors"
	"fmt"
	"strings"

	"github.com/Jinnrry/pmail/internal/domain"

	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

// GroupService 邮件分组用例：增删改查、路径层级与邮箱状态统计。
type GroupService struct {
	groups GroupPersistence
	stats  MailboxStatsPersistence
}

func NewGroupService(groups GroupPersistence, stats MailboxStatsPersistence) *GroupService {
	return &GroupService{groups: groups, stats: stats}
}

// GroupSvc 邮件分组服务实例，由 bootstrap.BindDependencies 装配
var GroupSvc *GroupService

type GroupItem struct {
	Id       int          `json:"id"`
	Label    string       `json:"label"`
	Tag      string       `json:"tag"`
	Children []*GroupItem `json:"children"`
}

type GroupIDRow = domain.GroupIDRow

func (s *GroupService) Create(ctx *context.Context, name string, parentId int) (*models.Group, error) {
	// 先查询是否存在
	var group models.Group
	s.groups.FindGroupByName(name, ctx.UserID, &group)
	if group.ID > 0 {
		return &group, nil
	}
	group.Name = name
	group.ParentId = parentId
	group.UserId = ctx.UserID
	group.FullPath = s.LayerName(ctx, &group, true)

	_, err := s.groups.InsertGroup(&group)

	return &group, err
}

func (s *GroupService) Rename(ctx *context.Context, oldName, newName string) error {
	oldGroupInfo, err := s.GetByName(ctx, oldName)
	if err != nil {
		return err
	}
	if oldGroupInfo == nil || oldGroupInfo.ID == 0 {
		return errors2.New("group not found")
	}
	oldGroupInfo.Name = newName
	oldGroupInfo.FullPath = s.LayerName(ctx, oldGroupInfo, true)
	_, err = s.groups.UpdateGroup(oldGroupInfo.ID, oldGroupInfo)
	return err
}

func (s *GroupService) GetByName(ctx *context.Context, name string) (*models.Group, error) {
	var group models.Group
	s.groups.FindGroupByName(name, ctx.UserID, &group)

	return &group, nil
}

func (s *GroupService) GetByFullPath(ctx *context.Context, fullPath string) (*models.Group, error) {
	var group models.Group
	_, err := s.groups.FindGroupByPath(fullPath, ctx.UserID, &group)

	return &group, err
}

func (s *GroupService) Delete(ctx *context.Context, groupId int) (bool, error) {
	allGroupIds := s.childIDs(ctx, groupId)
	allGroupIds = append(allGroupIds, groupId)

	return s.groups.RemoveGroups(ctx, allGroupIds)
}

func (s *GroupService) childIDs(ctx *context.Context, rootId int) []int {
	var ids []GroupIDRow
	var ret []int
	err := s.groups.FindChildGroupIDs(rootId, ctx.UserID, &ids)
	if err != nil {
		log.WithContext(ctx).Errorf("getAllChildId err: %v", err)
	}
	for _, item := range ids {
		ret = array.Merge(ret, s.childIDs(ctx, item.Id))
		ret = append(ret, item.Id)
	}
	return ret
}

// InfoList 获取全部的分组树
func (s *GroupService) InfoList(ctx *context.Context) []*GroupItem {
	return s.buildChildren(ctx, 0)
}

func (s *GroupService) buildChildren(ctx *context.Context, parentId int) []*GroupItem {
	var ret []*GroupItem
	var rootGroup []*models.Group
	err := s.groups.FindChildGroups(parentId, ctx.UserID, &rootGroup)

	if err != nil {
		log.WithContext(ctx).Errorf("SQL Error:%v", err)
	}

	for _, group := range rootGroup {
		ret = append(ret, &GroupItem{
			Id:       group.ID,
			Label:    group.Name,
			Tag:      dto.SearchTag{GroupId: group.ID, Status: -1, Type: -1}.ToString(),
			Children: s.buildChildren(ctx, group.ID),
		})
	}

	return ret
}

func (s *GroupService) List(ctx *context.Context) []*models.Group {
	var ret []*models.Group
	s.groups.ListUserGroups(ctx.UserID, &ret)
	return ret
}

func (s *GroupService) ListByPathPattern(ctx *context.Context, pattern string) []*models.Group {
	var ret []*models.Group
	s.groups.FindGroupsByPathPattern(ctx.UserID, pattern, &ret)
	return ret
}

func (s *GroupService) HasChildren(ctx *context.Context, id int) bool {
	var children []*models.Group
	s.groups.FindGroupsByParent(id, &children)

	return len(children) > 0
}

func (s *GroupService) LayerName(ctx *context.Context, item *models.Group, allPath bool) string {
	if item.ParentId == 0 {
		return item.Name
	}
	var parent models.Group
	_, _ = s.groups.FindGroupByID(item.ParentId, &parent)
	if allPath {
		return s.LayerName(ctx, &parent, allPath) + "/" + item.Name
	}
	return s.LayerName(ctx, &parent, allPath)
}

func IsDefaultBox(box string) bool {
	return array.InArray(box, []string{"INBOX", "Sent Messages", "Drafts", "Deleted Messages", "Junk"})
}

func (s *GroupService) Status(ctx *context.Context, groupName string, params []string) (string, map[string]int) {
	retMap := map[string]int{}

	if !IsDefaultBox(groupName) {
		groupNames := strings.Split(groupName, "/")
		groupName = groupNames[len(groupNames)-1]

		var group models.Group
		s.groups.FindUserGroupByName(ctx.UserID, groupName, &group)
		if group.ID == 0 {
			ret := ""
			for _, param := range params {
				if ret != "" {
					ret += " "
				}
				retMap[param] = 0
				ret += fmt.Sprintf("%s %d", param, 0)
			}
			return fmt.Sprintf("(%s)", ret), retMap
		}
		ret := ""
		for _, param := range params {
			if ret != "" {
				ret += " "
			}
			var value int

			switch param {
			case "MESSAGES":
				s.stats.CountGroupEmails(group.ID, &value)
			case "UIDNEXT":
				s.stats.FindGroupLastUID(group.ID, &value)
				value += 1
			case "UIDVALIDITY":
				value = group.ID
			case "UNSEEN":
				s.stats.CountGroupUnreadEmails(group.ID, &value)
			}
			retMap[param] = value
			ret += fmt.Sprintf("%s %d", param, value)
		}
		return fmt.Sprintf("(%s)", ret), retMap
	}

	ret := ""
	for _, param := range params {
		if ret != "" {
			ret += " "
		}
		var value int

		switch param {
		case "MESSAGES":
			value = s.groupNum(ctx, groupName, false)
		case "UIDNEXT":
			value = s.nextUID(ctx, groupName)
		case "UIDVALIDITY":
			value = models.GroupNameToCode[groupName]
		case "UNSEEN":
			value = s.groupNum(ctx, groupName, true)
		default:
			continue
		}
		retMap[param] = value
		ret += fmt.Sprintf("%s %d", param, value)
	}
	if ret == "" {
		return "", retMap
	}

	return fmt.Sprintf("(%s)", ret), retMap

}

// MessageCount 返回指定邮箱的邮件总数，供 IMAP IDLE 通知使用。
func (s *GroupService) MessageCount(ctx *context.Context, userID int, mailbox string) int {
	noticeCtx := *ctx
	noticeCtx.UserID = userID
	if mailbox == "" {
		mailbox = "INBOX"
	}
	_, data := s.Status(&noticeCtx, mailbox, []string{"MESSAGES"})
	return data["MESSAGES"]
}

func (s *GroupService) nextUID(ctx *context.Context, groupName string) int {
	var lastId int
	switch groupName {
	case "INBOX":
		s.stats.FindLastInboxUID(ctx.UserID, &lastId)
	case "Sent Messages":
		s.stats.FindLastSentUID(ctx.UserID, &lastId)
	case "Drafts":
		s.stats.FindLastDraftUID(ctx.UserID, &lastId)
	case "Deleted Messages":
		s.stats.FindLastDeletedUID(ctx.UserID, &lastId)
	case "Junk":
		s.stats.FindLastJunkUID(ctx.UserID, &lastId)
	}
	return lastId + 1
}

func (s *GroupService) groupNum(ctx *context.Context, groupName string, mustUnread bool) int {
	var count int
	switch groupName {
	case "INBOX":
		if mustUnread {
			s.stats.CountUnreadInboxEmails(ctx.UserID, &count)
		} else {
			s.stats.CountInboxEmails(ctx.UserID, &count)
		}
	case "Sent Messages":
		if mustUnread {
			count = 0
		} else {
			s.stats.CountSentEmails(ctx.UserID, &count)
		}
	case "Drafts":
		if mustUnread {
			s.stats.CountUnreadDraftEmails(ctx.UserID, &count)
		} else {
			s.stats.CountDraftEmails(ctx.UserID, &count)
		}
	case "Deleted Messages":
		if mustUnread {
			s.stats.CountUnreadDeletedEmails(ctx.UserID, &count)
		} else {
			s.stats.CountDeletedEmails(ctx.UserID, &count)
		}
	case "Junk":
		if mustUnread {
			s.stats.CountUnreadJunkEmails(ctx.UserID, &count)
		} else {
			s.stats.CountJunkEmails(ctx.UserID, &count)
		}
	}
	return count
}
