package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/kennygrant/sanitize"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/ziflex/lecho/v3"
	"golang.org/x/time/rate"

	"github.com/vanntile/go-for-the-truth/web/models"
	views "github.com/vanntile/go-for-the-truth/web/views"
)

func main() {
	config, err := NewAppConfig()
	if err != nil {
		log.Fatalf("failed to initialize app config: %v", err)
	}

	e := echo.New()
	e.HideBanner = true

	// Static assets
	e.Static("/", "web/public")

	// Logging setup
	var output io.Writer = zerolog.ConsoleWriter{Out: os.Stdout}
	if !config.development {
		output = os.Stdout
	}

	logger := lecho.New(output,
		lecho.WithLevel(log.DEBUG),
		lecho.WithTimestamp(),
		lecho.WithCaller(),
	)
	e.Logger = logger

	// Middleware
	if !config.development {
		e.Pre(middleware.HTTPSRedirect())
	}
	e.Use(middleware.RequestID())
	e.Use(lecho.Middleware(lecho.Config{
		Logger:      logger,
		HandleError: true,
	}))
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(10))))
	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		Skipper:            middleware.DefaultSkipper,
		XSSProtection:      "1; mode=block",
		ContentTypeNosniff: "nosniff",
		XFrameOptions:      "SAMEORIGIN",
		HSTSMaxAge:         31536000,
		HSTSPreloadEnabled: true,
		ContentSecurityPolicy: "default-src 'self'; style-src 'self'; script-src 'self'; base-uri 'self'; " +
			"worker-src 'none'; form-action 'self'; connect-src 'self'; object-src 'none'; media-src 'none'; " +
			"frame-ancestors 'none'; upgrade-insecure-requests;",
	}))
	e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		Skipper:      middleware.DefaultSkipper,
		ErrorMessage: "",
		Timeout:      30 * time.Second,
	}))
	e.Use(middleware.Gzip())
	e.Use(middleware.BodyLimitWithConfig(middleware.BodyLimitConfig{
		Skipper: func(e echo.Context) bool {
			return e.Path() == "/admin/questions"
		},
		Limit: "2K",
	}))
	e.Use(middleware.Recover())

	adminGroup := e.Group("/admin")
	adminGroup.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
		e.Logger.Infof("`%s` `%s`", password, config.adminPassword)
		if subtle.ConstantTimeCompare([]byte(username), []byte(config.adminUsername)) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte(config.adminPassword)) == 1 {
			return true, nil
		}

		e.Logger.Warnf("Authentication attempt failed for user `%s`", username)
		return false, nil
	}))

	// Database
	db, err := InitDB(config)
	if err != nil {
		e.Logger.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	e.Logger.Infof("Connected to database at %s", config.dbPath)

	// Handler
	count, err := GetQuestionCount(db)
	if err != nil {
		e.Logger.Fatalf("failed to get question count: %v", err)
	}
	h := &Handler{db: db, questionsPerQuiz: config.questionsPerQuiz, questionCount: count}
	// Routes
	e.GET("/", h.GetQuiz)
	e.POST("/", h.PostQuiz)
	adminGroup.GET("/management", h.GetAdminPage)
	adminGroup.POST("/questions", h.UploadQuestions)
	adminGroup.GET("/answers", h.GetAnswers)

	// Start server
	if config.development {
		e.Logger.Fatal(e.Start(config.address))
	} else {
		if err := e.StartTLS(config.address, config.crtPath, config.keyPath); err != http.ErrServerClosed {
			e.Logger.Fatal(err)
		}
	}
}

type AppConfig struct {
	development      bool
	address          string
	crtPath          string
	keyPath          string
	dbPath           string
	dbPassword       string
	adminUsername    string
	adminPassword    string
	questionsPerQuiz int
}

func NewAppConfig() (*AppConfig, error) {
	development := os.Getenv("GO_ENV") == "development"
	address := os.Getenv("ADDRESS")
	if address == "" {
		address = ":1323"
	}
	crtPath := os.Getenv("CERT_PATH")
	keyPath := os.Getenv("KEY_PATH")
	if !development {
		if crtPath == "" {
			return nil, fmt.Errorf("failed to read certificate path")
		}
		if keyPath == "" {
			return nil, fmt.Errorf("failed to read key path")
		}
	}
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		return nil, fmt.Errorf("failed to read DB URL")
	}
	dbPassword := os.Getenv("DATABASE_PASSWORD")
	if dbPassword == "" {
		return nil, fmt.Errorf("failed to read DB password")
	}
	adminUsername := os.Getenv("ADMIN_USERNAME")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminUsername == "" || adminPassword == "" {
		return nil, fmt.Errorf("failed to read admin credentials")
	}
	questionsPerQuiz, err := strconv.Atoi(os.Getenv("QUESTIONS_PER_QUIZ"))
	if err != nil {
		return nil, fmt.Errorf("failed to read question per quiz: %w", err)
	}

	return &AppConfig{
		development:      development,
		address:          address,
		crtPath:          crtPath,
		keyPath:          keyPath,
		dbPath:           dbPath,
		dbPassword:       dbPassword,
		adminUsername:    adminUsername,
		adminPassword:    adminPassword,
		questionsPerQuiz: questionsPerQuiz,
	}, nil
}

func firstN(s string, n int) string {
	v := []rune(s)
	if n >= len(v) {
		return s
	}
	return string(v[:n])
}

// Custom templ Render function
func Render(ctx echo.Context, statusCode int, t templ.Component) error {
	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)

	if err := t.Render(ctx.Request().Context(), buf); err != nil {
		return err
	}

	return ctx.HTML(statusCode, buf.String())
}

// DB operations

func InitDB(config *AppConfig) (*sql.DB, error) {
	sql.Register("sqlite3_ext", &sqlite3.SQLiteDriver{})
	dsn := fmt.Sprintf("%s?_cipher=sqlcipher&_key=%s", config.dbPath, url.QueryEscape(config.dbPassword))

	db, err := sql.Open("sqlite3_ext", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		tx.Rollback() //nolint:errcheck
	}()

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS questions " +
		"( ID INTEGER PRIMARY KEY AUTOINCREMENT, question TEXT, claims TEXT, fake INTEGER )")
	if err != nil {
		return nil, fmt.Errorf("failed to create questions table: %w", err)
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS answers " +
		"( created INTEGER, seed TEXT, real TEXT, fake TEXT, country TEXT, side TEXT, age TEXT )")
	if err != nil {
		return nil, fmt.Errorf("failed to create answers table: %w", err)
	}

	_, err = tx.Exec("CREATE INDEX IF NOT EXISTS answers_created ON answers(created)")
	if err != nil {
		return nil, fmt.Errorf("failed to create answers index: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return db, nil
}

func GetQuestionCount(db *sql.DB) (int, error) {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM questions").Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to read count from database: %w", err)
	}
	return count, nil
}

func ReadQuestionsByIDs(db *sql.DB, ids []int) ([]models.Question, error) {
	query := "SELECT id, question, claims, fake FROM questions" // no pagination needed on short data
	args := []interface{}{}

	if len(ids) > 0 { // read only some questions if ids are not nil
		for i := range ids {
			args = append(args, ids[i])
		}
		query = fmt.Sprintf( //nolint:gosec // not interpolating values, just syntax
			"SELECT id, question, claims, fake FROM questions WHERE ID IN (%s)",
			strings.Join(slices.Repeat([]string{"?"}, len(ids)), ","),
		)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	questions := make([]models.Question, 0, len(ids))
	for rows.Next() {
		var q models.Question
		if err := rows.Scan(&q.ID, &q.Question, &q.Claims, &q.Fake); err != nil {
			return nil, fmt.Errorf("failed scanning row: %w", err)
		}

		questions = append(questions, q)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed reading rows: %w", err)
	}

	return questions, nil
}

func SetQuestions(db *sql.DB, questions []models.Question) error {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		tx.Rollback() //nolint:errcheck
	}()
	_, err = tx.Exec("DELETE FROM questions")
	if err != nil {
		return fmt.Errorf("failed to delete questions: %w", err)
	}
	query := "INSERT INTO questions (ID, question, claims, fake) VALUES "
	args := []interface{}{}
	for i, question := range questions {
		if i != 0 {
			query += ", "
		}
		query += "(?,?,?,?)"
		args = append(args, question.ID, question.Question, question.Claims, question.Fake)
	}

	result, err := tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert questions: %w", err)
	}

	if affected, err := result.RowsAffected(); err != nil || int(affected) != len(questions) {
		return fmt.Errorf("failed to insert all questions: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}

func GetAnswers(db *sql.DB, limit int, offset *time.Time) ([]models.Answer, error) {
	query := "SELECT created, seed, real, fake, country, side, age FROM answers ORDER BY created LIMIT ?"
	args := []interface{}{limit}

	if offset != nil {
		query = "SELECT created, seed, real, fake, country, side, age " +
			"FROM answers WHERE created > ? ORDER BY created LIMIT ?"
		args = []interface{}{offset.UnixNano(), limit}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	answers := make([]models.Answer, 0, limit)
	for rows.Next() {
		var a models.Answer
		var createdNanoStr string
		if err := rows.Scan(&createdNanoStr, &a.Seed, &a.Real, &a.Fake,
			&a.Country, &a.Side, &a.Age); err != nil {
			return nil, fmt.Errorf("failed scanning row: %w", err)
		}

		createdNano, err := strconv.ParseInt(createdNanoStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed converting timestamp: `%s`: %w", createdNanoStr, err)
		}
		if createdNano < 0 {
			log.Errorf("failed to convert to a valid timestamp: `%s`", createdNanoStr)
			continue
		}
		a.Created = time.Unix(createdNano/(1e9), createdNano/(1e9))

		answers = append(answers, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed reading rows: %w", err)
	}

	return answers, nil
}

func AddAnswer(db *sql.DB, answer *models.Answer) error {
	query := "INSERT INTO answers (created, seed, real, fake, country, side, age) VALUES (?,?,?,?,?,?,?)"
	args := []interface{}{
		time.Now().UnixNano(), answer.Seed, answer.Real, answer.Fake, answer.Country, answer.Side, answer.Age,
	}

	result, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert an answer: %w", err)
	}

	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return fmt.Errorf("failed to insert a single answer: %w", err)
	}

	return nil
}

// Route handlers

type Handler struct {
	db               *sql.DB
	questionsPerQuiz int
	questionCount    int
}

func (h *Handler) GetQuiz(c echo.Context) error {
	// generate seed
	seed := rand.Uint64() //nolint:gosec // no cryptographic security needed, we publicly return the indices
	// generate random question indices
	r := rand.New(rand.NewPCG(1, seed)) //nolint:gosec
	ids := []int{}
	if h.questionCount > h.questionsPerQuiz {
		for len(ids) < h.questionsPerQuiz {
			candidate := r.IntN(1000000) % (h.questionCount)
			if !slices.Contains(ids, candidate) {
				ids = append(ids, candidate)
			}
		}
	}

	questions, err := ReadQuestionsByIDs(h.db, ids)
	if err != nil {
		c.Logger().Errorf("failed to read questions: %v", err)
		return Render(c, http.StatusAccepted, views.BaseLayout(views.ErrorPage("Failed to read questions")))
	}

	// Database returns the questions in ascending id order, so we need to manually shuffle
	r.Shuffle(len(questions), func(i, j int) {
		questions[i], questions[j] = questions[j], questions[i]
	})

	return Render(c, http.StatusOK, views.BaseLayout(
		views.QuizPage(fmt.Sprintf("%d", seed), false, questions, 0, nil),
	))
}

func (h *Handler) PostQuiz(c echo.Context) error {
	answer := models.Answer{
		Seed:    sanitize.HTML(firstN(c.FormValue("seed"), 64)),
		Real:    firstN(c.FormValue("real"), 256),
		Fake:    firstN(c.FormValue("fake"), 256),
		Country: firstN(c.FormValue("country"), 256),
		Side:    firstN(c.FormValue("side"), 64),
		Age:     firstN(c.FormValue("age"), 64),
	}

	// validate data
	var err error
	if len(answer.Real) > 0 {
		for _, x := range strings.Split(answer.Real, ",") {
			if id, convErr := strconv.Atoi(x); convErr != nil {
				err = fmt.Errorf("invalid real question ID: not a number: %w", convErr)
			} else if id < 0 || id >= h.questionCount {
				err = fmt.Errorf("invalid real question ID: must be between 0 and %d (inclusive)", h.questionCount-1)
			}
		}
	}
	if len(answer.Fake) > 0 {
		for _, x := range strings.Split(answer.Fake, ",") {
			if id, convErr := strconv.Atoi(x); convErr != nil {
				err = fmt.Errorf("invalid fake question ID: not a number: %w", convErr)
			} else if id < 0 || id >= h.questionCount {
				err = fmt.Errorf("invalid fake question ID: must be between 0 and %d (inclusive)", h.questionCount-1)
			}
		}
	}
	if !slices.Contains(models.COUNTRIES, answer.Country) {
		err = fmt.Errorf("invalid country: %s", answer.Country)
	}
	if !slices.Contains(models.GROUPS, answer.Side) {
		err = fmt.Errorf("invalid side: %s", answer.Side)
	}
	if age, convErr := strconv.Atoi(answer.Age); convErr != nil {
		err = fmt.Errorf("invalid age: not a number: %w", convErr)
	} else if age < 18 || age > 120 {
		err = fmt.Errorf("invalid age: must be between 18 and 120 (inclusive)")
	}

	if err != nil {
		c.Logger().Errorf("quiz validation error: %v", err)
		return Render(c, http.StatusAccepted, views.BaseLayout(views.ErrorPage("Invalid quiz response")))
	}

	answeredRealIDs := []int{}
	for _, s := range strings.Split(answer.Real, ",") {
		if id, err := strconv.Atoi(s); err == nil {
			answeredRealIDs = append(answeredRealIDs, id)
		}
	}
	answeredFakeIDs := []int{}
	for _, s := range strings.Split(answer.Fake, ",") {
		if id, err := strconv.Atoi(s); err == nil {
			answeredFakeIDs = append(answeredFakeIDs, id)
		}
	}
	ids := slices.Concat(answeredRealIDs, answeredFakeIDs)

	if err := AddAnswer(h.db, &answer); err != nil {
		c.Logger().Errorf("failed to add answer: %v", err)
		return Render(c, http.StatusAccepted, views.BaseLayout(views.ErrorPage("Failed to save answers")))
	}
	questions, err := ReadQuestionsByIDs(h.db, ids)
	if err != nil {
		c.Logger().Errorf("failed to read questions: %v", err)
		return Render(c, http.StatusAccepted, views.BaseLayout(views.ErrorPage("")))
	}

	// convert questions and answers to replies
	replies := []models.Reply{}
	for _, question := range questions {
		replies = append(replies, models.Reply{
			Fake: question.Fake, AnsweredFake: slices.Contains(answeredFakeIDs, question.ID),
			Question: question.Question, Claims: question.Claims,
		})
	}

	answeredCorrect := 0
	for _, reply := range replies {
		if reply.Fake == reply.AnsweredFake {
			answeredCorrect += 1
		}
	}

	return Render(c, http.StatusAccepted, views.BaseLayout(
		views.QuizPage(answer.Seed, true, nil, answeredCorrect, replies),
	))
}

func (h *Handler) GetAdminPage(c echo.Context) error {
	questions, err := ReadQuestionsByIDs(h.db, nil)
	if err != nil {
		c.Logger().Errorf("failed to read questions: %v", err)
		return Render(c, http.StatusAccepted, views.BaseLayout(views.ErrorPage("Failed to read questions")))
	}

	return Render(c, http.StatusOK, views.BaseLayout(views.AdminPage(questions)))
}

func (h *Handler) GetAnswers(c echo.Context) error {
	var offset *time.Time
	created := c.QueryParam("created")
	if created != "" {
		if t, err := time.Parse(time.RFC3339Nano, created); err == nil {
			offset = &t
		}
	}

	answers, err := GetAnswers(h.db, 4000, offset)
	if err != nil {
		c.Logger().Errorf("failed to read answers: %v", err)
		return Render(c, http.StatusAccepted, views.BaseLayout(views.ErrorPage("Failed to read answers")))
	}

	answerStr := ""
	for i := range answers {
		answerStr += fmt.Sprintf("%s,%s,\"%s\",%s,%s,\"%s\",\"%s\"\n", answers[i].Created.Format(time.RFC3339Nano),
			answers[i].Seed, answers[i].Country, answers[i].Side, answers[i].Age, answers[i].Fake, answers[i].Real)
	}

	return c.String(http.StatusOK, answerStr)
}

func (h *Handler) UploadQuestions(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		c.Logger().Errorf("failed to read file: %v", err)
		return c.String(http.StatusBadRequest, "Invalid form submission")
	}
	src, err := file.Open()
	if err != nil {
		c.Logger().Errorf("failed to open file: %v", err)
		return c.String(http.StatusBadRequest, "Failed to read input file")
	}
	defer src.Close()

	var content bytes.Buffer
	if _, err := (&content).ReadFrom(src); err != nil {
		return c.String(http.StatusBadRequest, "Failed to read input file")
	}

	questions := []models.Question{}
	for i, row := range strings.Split(content.String(), "\n") {
		if i == 0 && strings.HasPrefix(row, "ID") {
			continue
		}

		if strings.Count(row, ",") < 3 {
			continue
		}

		columns := strings.Split(row, ",")
		id, err := strconv.Atoi(columns[0])
		if err != nil {
			c.Logger().Errorf("failed to parse ID in row: `%s`", firstN(row, 1024))
			return c.String(http.StatusBadRequest, "Failed to validate question IDs")
		}
		claims := models.Group1
		if columns[1] == models.Group2 {
			claims = models.Group2
		}
		fake := true
		if columns[2] == "real" {
			fake = false
		}
		question := strings.Join(columns[3:], ",") // avoid losing content on commas inside questions
		question, _ = strings.CutPrefix(question, "\"")
		question, _ = strings.CutSuffix(question, "\"")
		questions = append(questions, models.Question{
			ID:       id,
			Claims:   claims,
			Fake:     fake,
			Question: sanitize.HTML(question),
		})
	}

	if err = SetQuestions(h.db, questions); err != nil {
		c.Logger().Errorf("failed to set new questions: %v", err)
		return c.String(http.StatusBadRequest, "Failed to update questions")
	}

	h.questionCount = len(questions)

	return c.NoContent(http.StatusOK)
}
