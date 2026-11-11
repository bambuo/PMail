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

type GroupItem struct {
	Id       int          `json:"id"`
	Label    string       `json:"label"`
	Tag      string       `json:"tag"`
	Children []*GroupItem `json:"children"`
}

func CreateGroup(ctx *context.Context, name string, parentId int) (*models.Group, error) {
	// 先查询是否存在
	var group models.Group
	Persistence.FindGroupByName(name, ctx.UserID, &group)
	if group.ID > 0 {
		return &group, nil
	}
	group.Name = name
	group.ParentId = parentId
	group.UserId = ctx.UserID
	group.FullPath = groupGetLayerName(ctx, &group, true)

	_, err := Persistence.InsertGroup(&group)

	return &group, err
}

func GroupRename(ctx *context.Context, oldName, newName string) error {
	oldGroupInfo, err := GroupGetGroupByName(ctx, oldName)
	if err != nil {
		return err
	}
	if oldGroupInfo == nil || oldGroupInfo.ID == 0 {
		return errors2.New("group not found")
	}
	oldGroupInfo.Name = newName
	oldGroupInfo.FullPath = groupGetLayerName(ctx, oldGroupInfo, true)
	_, err = Persistence.UpdateGroup(oldGroupInfo.ID, oldGroupInfo)
	return err
}

func GroupGetGroupByName(ctx *context.Context, name string) (*models.Group, error) {
	var group models.Group
	Persistence.FindGroupByName(name, ctx.UserID, &group)

	return &group, nil
}

func GroupGetGroupByFullPath(ctx *context.Context, fullPath string) (*models.Group, error) {
	var group models.Group
	_, err := Persistence.FindGroupByPath(fullPath, ctx.UserID, &group)

	return &group, err
}

func GroupDelGroup(ctx *context.Context, groupId int) (bool, error) {
	allGroupIds := groupGetAllChildId(ctx, groupId)
	allGroupIds = append(allGroupIds, groupId)

	return RemoveGroups(ctx, allGroupIds)
}

type GroupIDRow = domain.GroupIDRow

func groupGetAllChildId(ctx *context.Context, rootId int) []int {
	var ids []GroupIDRow
	var ret []int
	err := Persistence.FindChildGroupIDs(rootId, ctx.UserID, &ids)
	if err != nil {
		log.WithContext(ctx).Errorf("getAllChildId err: %v", err)
	}
	for _, item := range ids {
		ret = array.Merge(ret, groupGetAllChildId(ctx, item.Id))
		ret = append(ret, item.Id)
	}
	return ret
}

// GetGroupInfoList 获取全部的分组
func GroupGetGroupInfoList(ctx *context.Context) []*GroupItem {
	return groupBuildChildren(ctx, 0)
}

// MoveMailToGroup 将某封邮件移动到某个分组中
func GroupMoveMailToGroup(ctx *context.Context, mailId []int, groupId int) bool {
	res, err := Persistence.MoveEmailsToGroup(ctx, mailId, groupId, ctx.UserID)
	if err != nil {
		log.WithContext(ctx).Errorf("SQL Error:%+v", err)
		return false
	}
	rowNum, err := res.RowsAffected()
	if err != nil {
		log.WithContext(ctx).Errorf("SQL Error:%+v", err)
		return false
	}

	return rowNum > 0
}

func groupBuildChildren(ctx *context.Context, parentId int) []*GroupItem {
	var ret []*GroupItem
	var rootGroup []*models.Group
	err := Persistence.FindChildGroups(parentId, ctx.UserID, &rootGroup)

	if err != nil {
		log.WithContext(ctx).Errorf("SQL Error:%v", err)
	}

	for _, group := range rootGroup {
		ret = append(ret, &GroupItem{
			Id:       group.ID,
			Label:    group.Name,
			Tag:      dto.SearchTag{GroupId: group.ID, Status: -1, Type: -1}.ToString(),
			Children: groupBuildChildren(ctx, group.ID),
		})
	}

	return ret

}

func GroupGetGroupList(ctx *context.Context) []*models.Group {
	var ret []*models.Group
	Persistence.ListUserGroups(ctx.UserID, &ret)
	return ret
}

func groupHasChildren(ctx *context.Context, id int) bool {
	var parent []*models.Group
	Persistence.FindGroupsByParent(id, &parent)

	return len(parent) > 0
}

func groupGetLayerName(ctx *context.Context, item *models.Group, allPath bool) string {
	if item.ParentId == 0 {
		return item.Name
	}
	var parent models.Group
	_, _ = Persistence.FindGroupByID(item.ParentId, &parent)
	if allPath {
		return groupGetLayerName(ctx, &parent, allPath) + "/" + item.Name
	}
	return groupGetLayerName(ctx, &parent, allPath)
}

func GroupIsDefaultBox(box string) bool {
	return array.InArray(box, []string{"INBOX", "Sent Messages", "Drafts", "Deleted Messages", "Junk"})
}

func GetGroupStatus(ctx *context.Context, groupName string, params []string) (string, map[string]int) {
	retMap := map[string]int{}

	if !GroupIsDefaultBox(groupName) {
		groupNames := strings.Split(groupName, "/")
		groupName = groupNames[len(groupNames)-1]

		var group models.Group
		Persistence.FindUserGroupByName(ctx.UserID, groupName, &group)
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
				Persistence.CountGroupEmails(group.ID, &value)
			case "UIDNEXT":
				Persistence.FindGroupLastUID(group.ID, &value)
				value += 1
			case "UIDVALIDITY":
				value = group.ID
			case "UNSEEN":
				Persistence.CountGroupUnreadEmails(group.ID, &value)
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
			value = groupGetGroupNum(ctx, groupName, false)
		case "UIDNEXT":
			value = groupGetNextUID(ctx, groupName)
		case "UIDVALIDITY":
			value = models.GroupNameToCode[groupName]
		case "UNSEEN":
			value = groupGetGroupNum(ctx, groupName, true)
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

func groupGetNextUID(ctx *context.Context, groupName string) int {
	var lastId int
	switch groupName {
	case "INBOX":
		Persistence.FindLastInboxUID(ctx.UserID, &lastId)
	case "Sent Messages":
		Persistence.FindLastSentUID(ctx.UserID, &lastId)
	case "Drafts":
		Persistence.FindLastDraftUID(ctx.UserID, &lastId)
	case "Deleted Messages":
		Persistence.FindLastDeletedUID(ctx.UserID, &lastId)
	case "Junk":
		Persistence.FindLastJunkUID(ctx.UserID, &lastId)
	}
	return lastId + 1
}

func groupGetGroupNum(ctx *context.Context, groupName string, mustUnread bool) int {
	var count int
	switch groupName {
	case "INBOX":
		if mustUnread {
			Persistence.CountUnreadInboxEmails(ctx.UserID, &count)
		} else {
			Persistence.CountInboxEmails(ctx.UserID, &count)
		}
	case "Sent Messages":
		if mustUnread {
			count = 0
		} else {
			Persistence.CountSentEmails(ctx.UserID, &count)
		}
	case "Drafts":
		if mustUnread {
			Persistence.CountUnreadDraftEmails(ctx.UserID, &count)
		} else {
			Persistence.CountDraftEmails(ctx.UserID, &count)
		}
	case "Deleted Messages":
		if mustUnread {
			Persistence.CountUnreadDeletedEmails(ctx.UserID, &count)
		} else {
			Persistence.CountDeletedEmails(ctx.UserID, &count)
		}
	case "Junk":
		if mustUnread {
			Persistence.CountUnreadJunkEmails(ctx.UserID, &count)
		} else {
			Persistence.CountJunkEmails(ctx.UserID, &count)
		}
	}
	return count
}

func GroupMove2DefaultBox(ctx *context.Context, mailIds []int, groupName string) error {
	switch groupName {
	case "Deleted Messages":
		err := EmailDeleteDelEmail(ctx, mailIds, false)
		if err != nil {
			return err
		}
	case "INBOX":
		_, err := Persistence.MoveEmailsToInbox(ctx.UserID, mailIds)

		return err
	case "Sent Messages":
		_, err := Persistence.MoveEmailsToSent(ctx.UserID, mailIds)

		return err
	case "Drafts":
		_, err := Persistence.MoveEmailsToDraft(ctx.UserID, mailIds)

		return err
	case "Junk":
		_, err := Persistence.MoveEmailsToJunk(ctx.UserID, mailIds)

		return err
	}
	return nil
}
