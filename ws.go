package main

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/gorilla/websocket"
)

const devID = "4d9c39a8a93746b6db53675800021501"

// return hex-encoded string
func md5Sum(payload string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(payload)))
}

func decode(payload string) map[string]string {
	result := make(map[string]string)

	for _, item := range strings.Split(payload, "/") {
		parts := strings.Split(item, "@=")

		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}

	return result
}

func encode(data map[string]interface{}) string {
	var buf strings.Builder
	first := true

	for k, v := range data {
		if !first {
			buf.WriteString("/")
		}

		buf.WriteString(fmt.Sprintf("%s@=%s", k, v))
		first = false
	}

	return buf.String()
}

func genPayload(data map[string]interface{}) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0xb1, 0x02, 0x00, 0x00})

	buf.WriteString(encode(data))
	buf.WriteString("/\x00")

	length := buf.Len() - 4
	tmp := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmp, uint32(length))

	result := buf.Bytes()

	result[0] = tmp[0]
	result[1] = tmp[1]
	result[2] = tmp[2]
	result[3] = tmp[3]
	result[4] = tmp[0]
	result[5] = tmp[1]
	result[6] = tmp[2]
	result[7] = tmp[3]

	return result
}

func getFollowedCount(roomID string) (fontID string, obfuscatedNumber string, err error) {
	u := url.URL{Scheme: "wss", Host: "wsproxy.douyu.com:6672", Path: "/"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)

	if err != nil {
		err = errors.Wrap(err, "websocket could not dial")
		return
	}

	defer c.Close()

	rt := fmt.Sprintf("%d", time.Now().Unix())

	payload := genPayload(map[string]interface{}{
		"type":   "loginreq",
		"roomid": roomID,
		"devid":  devID,
		"rt":     rt,
		"vk":     md5Sum(rt + "r5*^5;}2#${XF[h+;'./.Q'1;,-]f'p[" + devID),
	})

	err = c.WriteMessage(websocket.BinaryMessage, payload)
	if err != nil {
		return
	}
	c.SetReadDeadline(time.Now().Add(20 * time.Second))

	done := make(chan struct{})

	go func() {
		defer close(done)

		msgCount := 0

		for {
			_, message, e := c.ReadMessage()

			if e != nil {
				err = errors.Wrap(e, "read message error")
				return
			}

			msgCount += 1

			// we need to retry
			if msgCount > 10 {
				err = errors.New("no followed_count received")
				return
			}

			data := decode(string(message[12:]))

			if data["type"] == "followed_count" {
				// log.Printf("recv: %v\n", data)
				fontID = data["ci"]
				obfuscatedNumber = data["cfdc"]
				return
			} else {
				// log.Printf("recv: %s\n", data["type"])
			}
		}
	}()

	<-done

	return
}
