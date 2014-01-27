// Copyright 2013 The StudyGolang Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// http://studygolang.com
// Author：polaris	studygolang@gmail.com

package service

import (
	"logger"
	"model"
	"strconv"
	"strings"
	"util"
)

// from给to发短信息
func SendMessageTo(from, to int, content string) bool {
	message := model.NewMessage()
	message.From = from
	message.To = to
	message.Content = content
	if _, err := message.Insert(); err != nil {
		logger.Errorln("message service SendMessageTo Error:", err)
		return false
	}

	// 通过 WebSocket 通知对方
	msg := NewMessage(WsMsgNotify, 1)
	go Book.PostMessage(to, msg)
	return true
}

// 给某人发系统消息
// to=0时，自己根据ext中的objid和objtype获得to
func SendSystemMsgTo(to, msgtype int, ext map[string]interface{}) bool {
	if to == 0 {
		objid := ext["objid"].(int)
		objtype := ext["objtype"].(int)
		switch objtype {
		case model.TYPE_TOPIC:
			to = getTopicOwner(objid)
		case model.TYPE_BLOG:
		case model.TYPE_RESOURCE:
			to = getResourceOwner(objid)
		case model.TYPE_WIKI:
			to = getWikiOwner(objid)
		}
	}
	if from, ok := ext["uid"]; ok {
		// 自己的动作不发系统消息
		if to == from.(int) {
			return true
		}
	}
	message := model.NewSystemMessage()
	message.To = to
	message.Msgtype = msgtype
	message.SetExt(ext)
	if _, err := message.Insert(); err != nil {
		logger.Errorln("message service SendSystemMsgTo Error:", err)
		return false
	}
	// 通过 WebSocket 通知对方
	msg := NewMessage(WsMsgNotify, 1)
	go Book.PostMessage(to, msg)
	return true
}

// 给被@的用户发系统消息
func SendSysMsgAtUids(uids string, ext map[string]interface{}) bool {
	if uids == "" {
		return true
	}
	message := model.NewSystemMessage()
	message.Msgtype = model.MsgtypeAtMe
	message.SetExt(ext)

	msg := NewMessage(WsMsgNotify, 1)

	uidSlice := strings.Split(uids, ",")
	for _, uidStr := range uidSlice {
		uid, _ := strconv.Atoi(strings.TrimSpace(uidStr))
		if from, ok := ext["uid"]; ok {
			// 自己的动作不发系统消息
			if uid == from.(int) {
				continue
			}
		}
		message.To = uid
		if _, err := message.Insert(); err != nil {
			logger.Errorln("message service SendSysMsgAtUids Error:", err)
			continue
		}
		// 通过 WebSocket 通知对方
		go Book.PostMessage(uid, msg)
	}
	return true
}

// 获得某人的系统消息
// 系统消息类型不同，在ext中存放的字段也不一样，如下：
//   model.MsgtypeTopicReply/MsgtypeResourceComment/MsgtypeWikiComment存放都为：
//		{"uid":xxx,"objid":xxx}
//   model.MsgtypeAtMe为：{"uid":xxx,"cid":xxx,"objid":xxx,"objtype":xxx}
func FindSysMsgsByUid(uid string) []map[string]interface{} {
	messages, err := model.NewSystemMessage().Where("to=" + uid).Order("ctime DESC").FindAll()
	if err != nil {
		logger.Errorln("message service FindSysMsgsByUid Error:", err)
		return nil
	}
	uids := make(map[int]int)
	tids := make(map[int]int)
	resIds := make(map[int]int)
	wikiIds := make(map[int]int)
	// 评论ID
	cids := make(map[int]int)

	ids := make([]int, 0, len(messages))
	for _, message := range messages {
		ext := message.Ext()
		if val, ok := ext["uid"]; ok {
			uid := int(val.(float64))
			uids[uid] = uid
		}
		var objid int
		if val, ok := ext["objid"]; ok {
			objid = int(val.(float64))
		}
		switch message.Msgtype {
		case model.MsgtypeTopicReply:
			tids[objid] = objid
		case model.MsgtypeResourceComment:
			resIds[objid] = objid
		case model.MsgtypeWikiComment:
			wikiIds[objid] = objid
		case model.MsgtypeAtMe:
			objTypeFloat := ext["objtype"].(float64)
			switch int(objTypeFloat) {
			case model.TYPE_TOPIC:
				tids[objid] = objid
			case model.TYPE_BLOG:
				//tids[objid] = objid
			case model.TYPE_RESOURCE:
				resIds[objid] = objid
			case model.TYPE_WIKI:
				wikiIds[objid] = objid
			}
		}
		if val, ok := ext["cid"]; ok {
			cid := int(val.(float64))
			cids[cid] = cid
		}
		if message.Hasread == "未读" {
			ids = append(ids, message.Id)
		}
	}
	// 标记已读
	go MarkHasRead(ids, true, util.MustInt(uid))

	userMap := getUserInfos(uids)
	commentMap := getComments(cids)
	topicMap := getTopics(tids)
	resourceMap := getResources(resIds)
	wikiMap := getWikis(wikiIds)

	result := make([]map[string]interface{}, len(messages))
	for i, message := range messages {
		tmpMap := make(map[string]interface{})
		// 某条信息的提示（标题）
		title := ""
		ext := message.Ext()
		if val, ok := ext["objid"]; ok {
			objTitle := ""
			objUrl := ""
			objid := int(val.(float64))
			switch message.Msgtype {
			case model.MsgtypeTopicReply:
				objTitle = topicMap[objid].Title
				objUrl = "/topics/" + strconv.Itoa(topicMap[objid].Tid)
				title = "回复了你的主题："
			case model.MsgtypeResourceComment:
				objTitle = resourceMap[objid].Title
				objUrl = "/resources/" + strconv.Itoa(resourceMap[objid].Id)
				title = "评论了你的资源："
			case model.MsgtypeWikiComment:
				objTitle = wikiMap[objid].Title
				objUrl = "/wiki/" + strconv.Itoa(wikiMap[objid].Id)
				title = "评论了你的Wiki页："
			case model.MsgtypeAtMe:
				title = "评论时提到了你，在"
				switch int(ext["objtype"].(float64)) {
				case model.TYPE_TOPIC:
					objTitle = topicMap[objid].Title
					objUrl = "/topics/" + strconv.Itoa(topicMap[objid].Tid)
					title += "主题："
				case model.TYPE_BLOG:
				case model.TYPE_RESOURCE:
					objTitle = resourceMap[objid].Title
					objUrl = "/resources/" + strconv.Itoa(resourceMap[objid].Id)
					title += "资源："
				case model.TYPE_WIKI:
					objTitle = wikiMap[objid].Title
					objUrl = "/wiki/" + strconv.Itoa(wikiMap[objid].Id)
					title += "wiki："
				}
			}
			tmpMap["objtitle"] = objTitle
			tmpMap["objurl"] = objUrl
		}
		tmpMap["ctime"] = message.Ctime
		tmpMap["id"] = message.Id
		tmpMap["hasread"] = message.Hasread
		if val, ok := ext["uid"]; ok {
			tmpMap["user"] = userMap[int(val.(float64))]
		}
		// content 和 cid不会同时存在
		if val, ok := ext["content"]; ok {
			tmpMap["content"] = val.(string)
		} else if val, ok := ext["cid"]; ok {
			tmpMap["content"] = decodeCmtContent(commentMap[int(val.(float64))])
		}
		tmpMap["title"] = title
		result[i] = tmpMap
	}
	return result
}

// 获得发给某人的短消息（收件箱）
func FindToMsgsByUid(uid string) []map[string]interface{} {
	messages, err := model.NewMessage().Where("to=" + uid + " AND tdel=" + model.TdelNotDel).Order("ctime DESC").FindAll()
	if err != nil {
		logger.Errorln("message service FindToMsgsByUid Error:", err)
		return nil
	}
	uids := make(map[int]int)
	ids := make([]int, 0, len(messages))
	for _, message := range messages {
		uids[message.From] = message.From
		if message.Hasread == model.NotRead {
			ids = append(ids, message.Id)
		}
	}
	// 标记已读
	go MarkHasRead(ids, false, util.MustInt(uid))
	userMap := getUserInfos(uids)
	result := make([]map[string]interface{}, len(messages))
	for i, message := range messages {
		tmpMap := make(map[string]interface{})
		util.Struct2Map(tmpMap, message)
		tmpMap["user"] = userMap[message.From]
		// 为了跟系统消息一致
		tmpMap["title"] = "发来了一条消息："
		result[i] = tmpMap
	}
	return result
}

// 获取某人发送的消息
func FindFromMsgsByUid(uid string) []map[string]interface{} {
	messages, err := model.NewMessage().Where("from=" + uid + " AND fdel=" + model.FdelNotDel).Order("ctime DESC").FindAll()
	if err != nil {
		logger.Errorln("message service FindFromMsgsByUid Error:", err)
		return nil
	}
	uids := make(map[int]int)
	for _, message := range messages {
		uids[message.To] = message.To
	}
	userMap := getUserInfos(uids)
	result := make([]map[string]interface{}, len(messages))
	for i, message := range messages {
		tmpMap := make(map[string]interface{})
		util.Struct2Map(tmpMap, message)
		tmpMap["user"] = userMap[message.To]
		result[i] = tmpMap
	}
	return result
}

// 标记消息已读
func MarkHasRead(ids []int, isSysMsg bool, uid int) bool {
	if len(ids) == 0 {
		return true
	}
	condition := "id=" + strconv.Itoa(ids[0])
	if len(ids) > 1 {
		condition = "id in(" + util.Join(ids, ",") + ")"
	}
	var err error
	if isSysMsg {
		err = model.NewSystemMessage().Set("hasread=" + model.HasRead).Where(condition).Update()
	} else {
		err = model.NewMessage().Set("hasread=" + model.HasRead).Where(condition).Update()
	}
	if err != nil {
		logger.Errorln("message service MarkHasRead Error:", err)
		return false
	}
	// 将显示的消息数减少
	msg := NewMessage(WsMsgNotify, -len(ids))
	go Book.PostMessage(uid, msg)
	return true
}

// 删除消息
// msgtype -> system(系统消息)/inbox(outbox)(短消息)
func DeleteMessage(id, msgtype string) bool {
	var err error
	if msgtype == "system" {
		err = model.NewSystemMessage().Where("id=" + id).Delete()
	} else if msgtype == "inbox" {
		// 打标记
		err = model.NewMessage().Set("tdel=" + model.TdelHasDel).Where("id=" + id).Update()
	} else {
		err = model.NewMessage().Set("fdel=" + model.FdelHasDel).Where("id=" + id).Update()
	}
	if err != nil {
		logger.Errorln("message service DeleteMessage Error:", err)
		return false
	}
	return true
}

// 获得某个用户未读消息数（系统消息和短消息）
func FindNotReadMsgNum(uid int) int {
	condition := "to=" + strconv.Itoa(uid) + " AND hasread=" + model.NotRead
	sysMsgNum, err := model.NewSystemMessage().Where(condition).Count()
	if err != nil {
		logger.Errorln("SystemMessage service FindNotReadMsgNum Error:", err)
	}
	msgNum, err := model.NewMessage().Where(condition).Count()
	if err != nil {
		logger.Errorln("Message service FindNotReadMsgNum Error:", err)
	}
	return sysMsgNum + msgNum
}
