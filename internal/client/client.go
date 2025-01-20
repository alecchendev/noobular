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

type ModuleInit struct {
	Title       string
	Description string
}

type BlockType int

const (
	QuestionBlockType BlockType = iota
	ContentBlockType
	KnowledgePointBlockType
)

func (b BlockType) String() string {
	switch b {
	case QuestionBlockType:
		return "question"
	case ContentBlockType:
		return "content"
	case KnowledgePointBlockType:
		return "knowledge_point"
	}
	return ""
}

type Block struct {
	BlockType BlockType
	Question  QuestionBlock
	Content   ContentBlock
}

func NewQuestionBlock(text string, choices []Choice, explanation string) Block {
	question := QuestionBlock{Text: text, Choices: choices, Explanation: explanation}
	return Block{BlockType: KnowledgePointBlockType, Question: question}
}

func NewContentBlock(content string) Block {
	return Block{BlockType: ContentBlockType, Content: ContentBlock{content}}
}

type QuestionBlock struct {
	Text        string
	Choices     []Choice
	Explanation string
}

type Choice struct {
	Text    string
	Correct bool
}

type ContentBlock struct {
	Text string
}

func CreateCourseRoute() string {
	return "/teacher/course/create"
}

func CreateCourseForm(title string, description string, public bool, modules []ModuleInit) url.Values {
	formData := url.Values{}
	formData.Set("title", title)
	formData.Set("description", description)
	if public {
		formData.Set("public", "on")
	}
	for _, module := range modules {
		formData.Add("module-title[]", module.Title)
		formData.Add("module-id[]", "-1")
		formData.Add("module-description[]", module.Description)
	}
	return formData
}

func (c Client) CreateCourse(title string, description string, public bool, modules []ModuleInit) *http.Response {
	formData := CreateCourseForm(title, description, public, modules)
	resp := c.post(CreateCourseRoute(), formData.Encode())
	return resp
}

func EditModuleRoute(courseId, moduleId int64) string {
	return fmt.Sprintf("/teacher/course/%d/module/%d", courseId, moduleId)
}

func addQuestionToForm(formData url.Values, question QuestionBlock) {
	formData.Add("question-title[]", question.Text)
	questionIdx := rand.Int()
	formData.Add("question-idx[]", strconv.Itoa(questionIdx))
	formData.Add("question-explanation[]", question.Explanation)
	for _, choice := range question.Choices {
		formData.Add("choice-title[]", choice.Text)
		choiceIdx := rand.Int()
		formData.Add("choice-idx[]", strconv.Itoa(choiceIdx))
		if choice.Correct {
			formData.Add("correct-choice-"+strconv.Itoa(questionIdx), strconv.Itoa(choiceIdx))
		}
	}
	formData.Add("choice-title[]", "end-choice")
	formData.Add("choice-idx[]", "end-choice")
}

func editModuleForm(title string, description string, blocks []Block) url.Values {
	formData := url.Values{}
	formData.Set("title", title)
	formData.Set("description", description)
	for _, block := range blocks {
		formData.Add("block-type[]", block.BlockType.String())
		switch block.BlockType {
		case KnowledgePointBlockType:
			question := block.Question
			addQuestionToForm(formData, question)
		case ContentBlockType:
			formData.Add("content-text[]", block.Content.Text)
		case QuestionBlockType:
			panic("QuestionBlockType not supported")
		}
	}
	return formData
}

func (c Client) EditModule(courseId int64, moduleId int64, title string, description string, blocks []Block) *http.Response {
	formData := editModuleForm(title, description, blocks)
	resp := c.put(EditModuleRoute(courseId, moduleId), formData.Encode())
	return resp
}

func (c Client) UploadModule(courseId int64, moduleId int64, module string) (*http.Response, error) {
	moduleTitle, moduleDescription, blocks, err := ParseModule(module)
	if err != nil {
		return nil, err
	}
	return c.EditModule(courseId, moduleId, moduleTitle, moduleDescription, blocks), nil
}

func ParseModule(module string) (string, string, []Block, error) {
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
	blocks := []Block{}
	questionBuffer := QuestionBlock{}

	finishPiece := func(parsingType int, newParsingType int, buffer []string, questionBuffer *QuestionBlock, blocks *[]Block) error {
		text := strings.Join(buffer, "\n")
		text = strings.TrimSpace(text)
		if parsingType == parsingContent {
			*blocks = append(*blocks, NewContentBlock(text))
		} else if parsingType == parsingQuestion {
			questionBuffer.Text = text
		} else if parsingType == parsingChoice {
			questionBuffer.Choices = append(questionBuffer.Choices, Choice{text, false})
		} else if parsingType == parsingCorrectChoice {
			questionBuffer.Choices = append(questionBuffer.Choices, Choice{text, true})
		} else if parsingType == parsingExplanation {
			questionBuffer.Explanation = text
		}

		if parsingType == parsingQuestion && !(newParsingType == parsingChoice || newParsingType == parsingCorrectChoice) {
			return fmt.Errorf("question must be followed by choice or correct choice")
		}

		justParsedChoice := parsingType == parsingChoice || parsingType == parsingCorrectChoice
		nextParsingNonQuestion := newParsingType != parsingChoice && newParsingType != parsingCorrectChoice && newParsingType != parsingExplanation
		justParsedExplanation := parsingType == parsingExplanation
		finishedQuestion := justParsedExplanation || (justParsedChoice && nextParsingNonQuestion)
		if finishedQuestion {
			*blocks = append(*blocks, NewQuestionBlock(questionBuffer.Text, questionBuffer.Choices, questionBuffer.Explanation))
			*questionBuffer = QuestionBlock{}
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
		default:
			continue // Comments
		}

		// If we matched a new block, it means we're at the end
		// of the previous block
		err := finishPiece(parsingType, newParsingType, buffer, &questionBuffer, &blocks)
		if err != nil {
			return "", "", nil, err
		}

		buffer = []string{}
		parsingType = newParsingType
	}

	err := finishPiece(parsingType, parsingNothing, buffer, &questionBuffer, &blocks)
	if err != nil {
		return "", "", nil, err
	}

	if err := scanner.Err(); err != nil {
		return "", "", nil, err
	}
	if metadataStatus != metadataParsed {
		return "", "", nil, fmt.Errorf("metadata not parsed")
	}

	return moduleTitle, moduleDescription, blocks, nil
}

func (c Client) CreateKnowledgePoint(courseId int64, name string, question QuestionBlock) *http.Response {
	formData := url.Values{}
	formData.Set("kp-name", name)
	addQuestionToForm(formData, question)
	resp := c.post(fmt.Sprintf("/teacher/course/%d/knowledge-point", courseId), formData.Encode())
	return resp
}
