package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/otiai10/gosseract"
	"github.com/pkg/errors"
	"github.com/veandco/go-sdl2/img"
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

var httpClient = &http.Client{
	Timeout: time.Minute,
}

var ocrClient = gosseract.NewClient()
var ocrLock sync.Mutex

type crawlResult struct {
	err              error
	roomID           string
	fontID           string
	obfuscatedNumber string
	realNumber       string // "" means OCR error
}

var mapping sync.Map

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func parseRoomIDsFromFile(p string) ([]string, error) {
	file, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var result []string

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		result = append(result, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func main() {
	startTime := time.Now()

	// prepare folders
	must(os.MkdirAll("result", 0755))
	must(os.MkdirAll("result/fonts", 0755))
	must(os.MkdirAll("result/tmp", 0755))

	defer ocrClient.Close()

	// init sdl ttf
	must(ttf.Init())

	if len(os.Args) == 1 {
		fmt.Println(`usage: douyu-crawler-demo [roomid]...
or douyu-crawler-demo -f <file>
or douyu-crawler-demo -ocr <font_file>`)
		return
	}

	if os.Args[1] == "-ocr" {
		fontPath := os.Args[2]
		imgPath := path.Join("result", "tmp", "tmp.png")

		must(renderFont(fontPath, imgPath))

		result := ocrText(imgPath)

		if isOCRValid(result) {
			fmt.Printf("ocr result: %s\n", result)
		} else {
			fmt.Printf("ocr faield")
		}

		return
	}

	// load previous mapping
	mappingFilePath := path.Join("result", "mapping.json")
	{
		if fileExists(mappingFilePath) {
			content, err := ioutil.ReadFile(mappingFilePath)
			must(err)

			var m map[string]string
			must(json.Unmarshal(content, &m))

			for k, v := range m {
				mapping.Store(k, v)
			}
		}
	}

	var roomIDs []string

	if os.Args[1] == "-f" {
		var err error
		roomIDs, err = parseRoomIDsFromFile(os.Args[2])
		if err != nil {
			log.Fatalf("could not parse room ids from file %s: %v", os.Args[2], err)
		}
	} else {
		roomIDs = os.Args[1:]
	}

	log.Printf("room ids: %v\n", roomIDs)

	var wg sync.WaitGroup

	totalCount := len(roomIDs)
	successCount := 0
	errorCount := 0
	ocrFailedCount := 0

	// csv file
	// fields: room id, font id, obfuscated number, real number
	// if ocr failed, write `?` for real number
	resultPath := path.Join("result", "result.txt")
	resultFile, err := os.Create(resultPath)
	must(err)
	defer resultFile.Close()

	roomIDChan := make(chan string)

	go func() {
		for _, roomID := range roomIDs {
			roomIDChan <- roomID
		}

		close(roomIDChan)
	}()

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for roomID := range roomIDChan {
				l := log.New(os.Stdout, fmt.Sprintf("[%s] ", roomID), log.Ltime)

				// l.SetOutput(ioutil.Discard)
				result := crawlRoom(l, roomID)
				// l.SetOutput(os.Stdout)

				if result.err != nil {
					l.Printf("error: %v\n", result.err)
					errorCount += 1
					continue
				}

				if result.realNumber == "" {
					l.Printf("ocr failed, need manually processing: %s\n", result.fontID)
					ocrFailedCount += 1
				} else {
					successCount += 1
				}

				l.Println("success")

				if result.realNumber == "" {
					result.realNumber = "?"
				}
				resultFile.WriteString(fmt.Sprintf("%s,%s,%s,%s\n",
					roomID,
					result.fontID,
					result.obfuscatedNumber,
					result.realNumber,
				))
			}
		}()
	}

	wg.Wait()

	// store mapping
	{
		f, err := os.Create(mappingFilePath)
		must(err)
		defer f.Close()

		m := make(map[string]string)

		mapping.Range(func(key, value interface{}) bool {
			m[key.(string)] = value.(string)
			return true
		})

		buf, err := json.MarshalIndent(m, "", "  ")
		must(err)

		f.Write(buf)
	}

	log.Printf("all done in %v\n", time.Since(startTime))
	log.Printf("  total: %d\n", totalCount)
	log.Printf("  success: %d\n", successCount)
	log.Printf("  error: %d\n", errorCount)
	log.Printf("  ocr failed count: %d\n", ocrFailedCount)
}

func crawlRoom(l *log.Logger, roomID string) crawlResult {
	l.Println("start to crawl followed count")

	result := crawlResult{roomID: roomID}

	// fetch followed_count, retry 5 times
	{
		var f, o string
		var err error

		for i := 0; i < 10; i++ {
			if i != 0 {
				l.Printf("retry for error %v\n", err)
				time.Sleep(time.Duration(i) * time.Second)
			}

			f, o, err = getFollowedCount(roomID)

			if f != "" {
				result.fontID = f
				result.obfuscatedNumber = o
				break
			}
		}

		if result.fontID == "" {
			result.err = errors.Wrap(err, "could not get followed_count")
			return result
		}

		l.Printf("followed_count fetched, fontID: %s, obfuscatedNumber: %s\n", result.fontID, result.obfuscatedNumber)
	}

	// check if we already have mapping
	if v, ok := mapping.Load(result.fontID); ok {
		l.Println("mapipng found")

		result.realNumber = parseObfuscatedNumber(result.obfuscatedNumber, v.(string))
		return result
	}

	// download font
	fontPath := path.Join("result", "fonts", result.fontID+".woff")
	{
		if !fileExists(fontPath) {
			if err := downloadFont(result.fontID, fontPath); err != nil {
				result.err = errors.Wrap(err, "could not download font file")
			}
			l.Println("font downloaded")
		} else {
			l.Println("font found")
		}
	}

	// render font
	imgPath := path.Join("result", "tmp", result.fontID+".png")
	{
		if fileExists(imgPath) {
			if err := renderFont(fontPath, imgPath); err != nil {
				result.err = errors.Wrap(err, "could not render font")
				return result
			}

			l.Println("rendered image created")
		} else {
			l.Println("rendered image found")
		}
	}

	// ocr font
	ocrResult := ocrText(imgPath)

	if !isOCRValid(ocrResult) {
		l.Println("could not recognize font")
		return result
	}

	l.Printf("font recognized, mapping: %s\n", ocrResult)
	mapping.Store(result.fontID, ocrResult)

	result.realNumber = parseObfuscatedNumber(result.obfuscatedNumber, ocrResult)

	l.Printf("real number parsed: %s\n", result.realNumber)

	return result
}

// https://shark.douyucdn.cn/app/douyu/res/font/FONT_ID.woff
func downloadFont(fontID string, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := httpClient.Get(fmt.Sprintf("https://shark.douyucdn.cn/app/douyu/res/font/%s.woff", fontID))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func renderFont(fontPath, imgPath string) error {
	font, err := ttf.OpenFont(fontPath, 50)

	if err != nil {
		return errors.Wrap(err, "could not open font")
	}

	surface, err := font.RenderUTF8Solid("0123456789", sdl.Color{0, 0, 0, 0})
	if err != nil {
		return errors.Wrap(err, "could not render text")
	}

	return img.SavePNG(surface, imgPath)
}

func ocrText(imgPath string) string {
	ocrLock.Lock()
	defer ocrLock.Unlock()

	ocrClient.SetWhitelist("0123456789")
	ocrClient.SetImage(imgPath)
	text, _ := ocrClient.Text()
	return text
}

// should a string of length 10 with 0 ~ 9
func isOCRValid(str string) bool {
	return len(str) == 10 &&
		strings.ContainsRune(str, '0') &&
		strings.ContainsRune(str, '1') &&
		strings.ContainsRune(str, '2') &&
		strings.ContainsRune(str, '3') &&
		strings.ContainsRune(str, '4') &&
		strings.ContainsRune(str, '5') &&
		strings.ContainsRune(str, '6') &&
		strings.ContainsRune(str, '7') &&
		strings.ContainsRune(str, '8') &&
		strings.ContainsRune(str, '9')
}

func parseObfuscatedNumber(o, mapping string) string {
	var result strings.Builder

	for _, digit := range o {
		result.WriteByte(mapping[digit-'0'])
	}

	return result.String()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
