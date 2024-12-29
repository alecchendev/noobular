package client

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type Client struct {
	baseUrl       string
	session_token *http.Cookie
}

func NewClient(baseUrl string, session_token *http.Cookie) Client {
	return Client{baseUrl, session_token}
}

func (c Client) request(method string, path string, body string) *http.Response {
	req, _ := http.NewRequest(method, c.baseUrl+path, strings.NewReader(body))
	if method == "POST" || method == "PUT" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if c.session_token != nil {
		req.AddCookie(c.session_token)
	}
	resp, _ := http.DefaultClient.Do(req)
	return resp
}

func (c Client) get(path string) *http.Response {
	return c.request("GET", path, "")
}

func (c Client) post(path string, body string) *http.Response {
	return c.request("POST", path, body)
}

func (c Client) put(path string, body string) *http.Response {
	return c.request("PUT", path, body)
}

func (c Client) delete(path string) *http.Response {
	return c.request("DELETE", path, "")
}

type blockType int

const (
	questionBlockType blockType = iota
	contentBlockType
)

func (b blockType) String() string {
	switch b {
	case questionBlockType:
		return "question"
	case contentBlockType:
		return "content"
	}
	return ""
}

type block struct {
	blockType blockType
	question  questionBlock
	content   contentBlock
}

func newQuestionBlock(text string, choices []choice, explanation string) block {
	question := questionBlock{text: text, choices: choices, explanation: explanation}
	return block{blockType: questionBlockType, question: question}
}

func newContentBlock(content string) block {
	return block{blockType: contentBlockType, content: contentBlock{content}}
}

type questionBlock struct {
	text       string
	choices    []choice
	explanation string
}

type choice struct {
	text    string
	correct bool
}

type contentBlock struct {
	text string
}

func editModuleRoute(courseId, moduleId int64) string {
	return fmt.Sprintf("/teacher/course/%d/module/%d", courseId, moduleId)
}

func editModuleForm(title string, description string, blocks []block) url.Values {
	formData := url.Values{}
	formData.Set("title", title)
	formData.Set("description", description)
	for _, block := range blocks {
		formData.Add("block-type[]", block.blockType.String())
		switch block.blockType {
		case questionBlockType:
			question := block.question
			formData.Add("question-title[]", question.text)
			questionIdx := rand.Int()
			formData.Add("question-idx[]", strconv.Itoa(questionIdx))
			formData.Add("question-explanation[]", question.explanation)
			for _, choice := range question.choices {
				formData.Add("choice-title[]", choice.text)
				choiceIdx := rand.Int()
				formData.Add("choice-idx[]", strconv.Itoa(choiceIdx))
				if choice.correct {
					formData.Add("correct-choice-"+strconv.Itoa(questionIdx), strconv.Itoa(choiceIdx))
				}
			}
			formData.Add("choice-title[]", "end-choice")
			formData.Add("choice-idx[]", "end-choice")
		case contentBlockType:
			formData.Add("content-text[]", block.content.text)
		}
	}
	return formData
}

func (c Client) UploadModule(courseId int64, moduleId int64, module string) (*http.Response, error) {
	moduleTitle, moduleDescription, blocks, err := ParseModule(module)
	if err != nil {
		return nil, err
	}
	formData := editModuleForm(moduleTitle, moduleDescription, blocks)
	resp := c.put(editModuleRoute(courseId, moduleId), formData.Encode())
	return resp, nil
}

func ParseModule(module string) (string, string, []block, error) {
	metadataUnseen := 0
	metadataProcessing := 1
	metadataParsed := 2
	metadataStatus := metadataUnseen
	moduleTitle := ""
	moduleDescription := ""
	parsingNothing := 0
	parsingContent := 1
	parsingQuestion := 2
	parsingChoice := 3
	parsingCorrectChoice := 4
	parsingExplanation := 5
	parsingType := parsingNothing
	buffer := []string{}
	blocks := []block{}
	questionBuffer := questionBlock{}

	finishPiece := func(parsingType int, newParsingType int, buffer []string, questionBuffer *questionBlock, blocks *[]block) error {
		text := strings.Join(buffer, "\n")
		text = strings.TrimSpace(text)
		if parsingType == parsingContent {
			*blocks = append(*blocks, newContentBlock(text))
		} else if parsingType == parsingQuestion {
			questionBuffer.text = text
		} else if parsingType == parsingChoice {
			questionBuffer.choices = append(questionBuffer.choices, choice{text, false})
		} else if parsingType == parsingCorrectChoice {
			questionBuffer.choices = append(questionBuffer.choices, choice{text, true})
		} else if parsingType == parsingExplanation {
			questionBuffer.explanation = text
		}

		if parsingType == parsingQuestion && !(newParsingType == parsingChoice || newParsingType == parsingCorrectChoice) {
			return fmt.Errorf("question must be followed by choice or correct choice")
		}

		justParsedChoice := parsingType == parsingChoice || parsingType == parsingCorrectChoice
		nextParsingNonQuestion := newParsingType != parsingChoice && newParsingType != parsingCorrectChoice && newParsingType != parsingExplanation
		justParsedExplanation := parsingType == parsingExplanation
		finishedQuestion := justParsedExplanation || (justParsedChoice && nextParsingNonQuestion)
		if finishedQuestion {
			*blocks = append(*blocks, newQuestionBlock(questionBuffer.text, questionBuffer.choices, questionBuffer.explanation))
			*questionBuffer = questionBlock{}
		}
		return nil
	}

	scanner := bufio.NewScanner(strings.NewReader(module))

	for scanner.Scan() {
		line := scanner.Text()
		if metadataStatus == metadataUnseen && line == "" {
			continue
		}
		if metadataStatus == metadataUnseen && line == "---" {
			metadataStatus = metadataProcessing
			continue
		}
		if metadataStatus == metadataProcessing && line == "---" {
			metadataStatus = metadataParsed
			continue
		}
		if metadataStatus == metadataProcessing {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) != 2 {
				return "", "", nil, fmt.Errorf("metadata not key value")
			}
			key := parts[0]
			value := parts[1]
			if key == "title" {
				moduleTitle = value
			} else if key == "description" {
				moduleDescription = value
			}
			continue
		}
		if metadataStatus != metadataParsed {
			return "", "", nil, fmt.Errorf("metadata not parsed")
		}

		pattern := `^\[//\]: # \((.+?)\)$`
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			buffer = append(buffer, line)
			continue
		}

		// The first element is the whole match, the second is the captured group
		parsedValue := matches[1]
		values := strings.Split(parsedValue, " ")
		valueType := values[0]
		newParsingType := parsingNothing
		switch valueType {
		case "content":
			newParsingType = parsingContent
		case "question":
			newParsingType = parsingQuestion
		case "choice":
			newParsingType = parsingChoice
			if len(values) == 2 && values[1] == "correct" {
				newParsingType = parsingCorrectChoice
			}
		case "explanation":
			newParsingType = parsingExplanation
		}

		// If we matched a new block, it means we're at the end
		// of the previous block
		finishPiece(parsingType, newParsingType, buffer, &questionBuffer, &blocks)

		buffer = []string{}
		parsingType = newParsingType
	}

	finishPiece(parsingType, parsingNothing, buffer, &questionBuffer, &blocks)

	if err := scanner.Err(); err != nil {
		return "", "", nil, err
	}
	if metadataStatus != metadataParsed {
		return "", "", nil, fmt.Errorf("metadata not parsed")
	}

	return moduleTitle, moduleDescription, blocks, nil
}
