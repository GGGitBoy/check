package send

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkauth "github.com/larksuite/oapi-sdk-go/v3/service/auth/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"time"
)

type Message struct {
	MsgType   string  `json:"msg_type"`
	Content   Content `json:"content"`
	Timestamp int64   `json:"timestamp"`
	Sign      string  `json:"sign"`
}

type Content struct {
	Text string `json:"text"`
}

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func Webhook(webhookURL, secret, text, taskName string) error {
	logrus.Infof("[%s] Start webhook send...", taskName)
	timestamp := time.Now().Unix()
	sign, err := GenSign(secret, timestamp)
	if err != nil {
		logrus.Errorf("Failed to get sign: %v", err)
		return fmt.Errorf("Failed to get sign: %v\n", err)
	}

	message := Message{
		MsgType: "text",
		Content: Content{
			Text: text,
		},
		Timestamp: timestamp,
		Sign:      sign,
	}

	// 将消息结构体转为 JSON
	messageBytes, err := json.Marshal(message)
	if err != nil {
		logrus.Errorf("Failed to marshal message: %v", err)
		return fmt.Errorf("Failed to marshal message: %v\n", err)
	}

	// 发送 POST 请求到飞书 Webhook
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(messageBytes))
	if err != nil {
		logrus.Errorf("Failed to send message: %v", err)
		return fmt.Errorf("Failed to send message: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logrus.Errorf("Failed to send message, status code: %d\n", resp.StatusCode)
		return fmt.Errorf("Failed to send message, status code: %d\n", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Failed to read resp body: %v\n", err)
		return fmt.Errorf("Failed to read resp body: %v\n", err)
	}

	response := &Response{}
	err = json.Unmarshal(body, response)
	if err != nil {
		logrus.Errorf("Failed to unmarshal resp body: %v\n", err)
		return fmt.Errorf("Failed to unmarshal resp body: %v\n", err)
	}

	if response.Code != 0 {
		logrus.Errorf("Failed to send message, code: %d msg: %s \n", response.Code, response.Msg)
		return fmt.Errorf("Failed to send message, code: %d msg: %s \n", response.Code, response.Msg)
	}

	logrus.Infof("[%s] Message sent successfully!", taskName)
	return nil
}

func GenSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%v", timestamp) + "\n" + secret

	var data []byte
	h := hmac.New(sha256.New, []byte(stringToSign))
	_, err := h.Write(data)
	if err != nil {
		return "", err
	}

	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return signature, nil
}

func Notify(appID, appSecret, fileName, filePath, message, taskName string) error {
	logrus.Infof("[%s] Start notify send...", taskName)
	// 创建 Client
	client := lark.NewClient(appID, appSecret, lark.WithEnableTokenCache(false))

	getTokenReq := larkauth.NewInternalAppAccessTokenReqBuilder().
		Body(larkauth.NewInternalAppAccessTokenReqBodyBuilder().
			AppId(appID).
			AppSecret(appSecret).
			Build()).
		Build()

	getTokenResp, err := client.Auth.AppAccessToken.Internal(context.Background(), getTokenReq)
	if err != nil {
		logrus.Errorf("Error get token in Lark: %v", err)
		return fmt.Errorf("failed to get token in Lark: %v", err)
	}

	if !getTokenResp.Success() {
		logrus.Errorf("Server error get token: Code=%d, Msg=%s, RequestID=%s", getTokenResp.Code, getTokenResp.Msg, getTokenResp.RequestId())
		return fmt.Errorf("server error get token: Code=%d, Msg=%s, RequestID=%s", getTokenResp.Code, getTokenResp.Msg, getTokenResp.RequestId())
	}

	logrus.Debugf("Token got successfully: %s", larkcore.Prettify(getTokenResp))

	appAccessTokenResp := larkcore.AppAccessTokenResp{}
	err = json.Unmarshal(getTokenResp.RawBody, &appAccessTokenResp)
	if err != nil {
		logrus.Errorf("Failed to unmarshal request body: %v", err)
		return fmt.Errorf("Failed to unmarshal request body: %v\n", err)
	}

	// 添加 token 到请求头
	requestOptionFunc := larkcore.WithHeaders(http.Header{
		"Authorization": []string{fmt.Sprintf("Bearer %s", appAccessTokenResp.AppAccessToken)},
	})

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		logrus.Errorf("Error opening file %s: %v", filePath, err)
		return fmt.Errorf("failed to open file %s: %v", filePath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			logrus.Warnf("Error closing file %s: %v", filePath, err)
		}
	}()

	// 创建文件上传请求对象
	createFileReq := larkim.NewCreateFileReqBuilder().
		Body(larkim.NewCreateFileReqBodyBuilder().
			FileType(`pdf`).
			FileName(fileName).
			File(file).
			Build()).
		Build()

	// 发起文件上传请求
	createFileResp, err := client.Im.File.Create(context.Background(), createFileReq, requestOptionFunc)
	if err != nil {
		logrus.Errorf("Error creating file in Lark: %v", err)
		return fmt.Errorf("failed to create file in Lark: %v", err)
	}

	// 服务端错误处理
	if !createFileResp.Success() {
		logrus.Errorf("Server error creating file: Code=%d, Msg=%s, RequestID=%s", createFileResp.Code, createFileResp.Msg, createFileResp.RequestId())
		return fmt.Errorf("server error creating file: Code=%d, Msg=%s, RequestID=%s", createFileResp.Code, createFileResp.Msg, createFileResp.RequestId())
	}

	logrus.Debugf("File created successfully: %s", larkcore.Prettify(createFileResp))

	// 创建列表聊天请求对象
	listChatReq := larkim.NewListChatReqBuilder().
		SortType(`ByCreateTimeAsc`).
		PageSize(20).
		Build()

	// 发起列表聊天请求
	listChatResp, err := client.Im.Chat.List(context.Background(), listChatReq, requestOptionFunc)
	if err != nil {
		logrus.Errorf("Error listing chats in Lark: %v", err)
		return fmt.Errorf("failed to list chats in Lark: %v", err)
	}

	// 服务端错误处理
	if !listChatResp.Success() {
		logrus.Errorf("Server error listing chats: Code=%d, Msg=%s, RequestID=%s", listChatResp.Code, listChatResp.Msg, listChatResp.RequestId())
		return fmt.Errorf("server error listing chats: Code=%d, Msg=%s, RequestID=%s", listChatResp.Code, listChatResp.Msg, listChatResp.RequestId())
	}

	logrus.Debugf("Chats listed successfully: %s", larkcore.Prettify(listChatResp))

	// 循环发送消息到每个聊天
	var content Content
	content.Text = message
	data, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("Error marshaling Content data: %v\n", err)
	}

	for _, i := range listChatResp.Data.Items {
		// 发送文本消息
		createMessageReq := larkim.NewCreateMessageReqBuilder().
			ReceiveIdType(`chat_id`).
			Body(larkim.NewCreateMessageReqBodyBuilder().
				ReceiveId(*i.ChatId).
				MsgType(`text`).
				Content(string(data)).
				Build()).
			Build()

		createMessageResp, err := client.Im.Message.Create(context.Background(), createMessageReq, requestOptionFunc)
		if err != nil {
			logrus.Errorf("Error sending text message to chat %s: %v", *i.ChatId, err)
			return fmt.Errorf("failed to send text message to chat %s: %v", *i.ChatId, err)
		}

		if !createMessageResp.Success() {
			logrus.Errorf("Server error sending text message: Code=%d, Msg=%s, RequestID=%s", createMessageResp.Code, createMessageResp.Msg, createMessageResp.RequestId())
			return fmt.Errorf("server error sending text message: Code=%d, Msg=%s, RequestID=%s", createMessageResp.Code, createMessageResp.Msg, createMessageResp.RequestId())
		}

		logrus.Infof("[%s] Text message sent successfully to chat %s", taskName, *i.ChatId)

		// 发送文件消息
		createFileMessageReq := larkim.NewCreateMessageReqBuilder().
			ReceiveIdType(`chat_id`).
			Body(larkim.NewCreateMessageReqBodyBuilder().
				ReceiveId(*i.ChatId).
				MsgType(`file`).
				Content("{\"file_key\":\"" + *createFileResp.Data.FileKey + "\"}").
				Build()).
			Build()

		createFileMessageResp, err := client.Im.Message.Create(context.Background(), createFileMessageReq, requestOptionFunc)
		if err != nil {
			logrus.Errorf("Error sending file message to chat %s: %v", *i.ChatId, err)
			return fmt.Errorf("failed to send file message to chat %s: %v", *i.ChatId, err)
		}

		if !createFileMessageResp.Success() {
			logrus.Errorf("Server error sending file message: Code=%d, Msg=%s, RequestID=%s", createFileMessageResp.Code, createFileMessageResp.Msg, createFileMessageResp.RequestId())
			return fmt.Errorf("server error sending file message: Code=%d, Msg=%s, RequestID=%s", createFileMessageResp.Code, createFileMessageResp.Msg, createFileMessageResp.RequestId())
		}

		logrus.Infof("[%s] File message sent successfully to chat %s", taskName, *i.ChatId)
	}

	return nil
}
