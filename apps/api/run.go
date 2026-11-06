package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	"cloud.google.com/go/storage"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	gcpProject = os.Getenv("GCP_PROJECT")
	gcpRegion  = os.Getenv("GCP_REGION")
	jobName    = os.Getenv("RUNNER_JOB")
	bucketName = os.Getenv("RUNS_BUCKET")
)

var (
	jobsClient    *run.JobsClient
	storageClient *storage.Client
)

func setupGoogle(ctx context.Context) error {
	if gcpProject == "" || gcpRegion == "" || jobName == "" || bucketName == "" {
		return fmt.Errorf("set GCP_PROJECT, GCP_REGION, RUNNER_JOB and RUNS_BUCKET")
	}

	var err error
	jobsClient, err = run.NewJobsClient(ctx)
	if err != nil {
		return err
	}

	storageClient, err = storage.NewClient(ctx)
	return err
}

type runResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	TimedOut bool   `json:"timed_out"`
}

func uploadCode(ctx context.Context, path string, code string) error {
	w := storageClient.Bucket(bucketName).Object(path).NewWriter(ctx)
	if _, err := io.WriteString(w, code); err != nil {
		return err
	}

	return w.Close()
}

func startJob(ctx context.Context, runID string, language string) error {
	req := &runpb.RunJobRequest{
		Name: fmt.Sprintf("projects/%s/locations/%s/jobs/%s", gcpProject, gcpRegion, jobName),
		Overrides: &runpb.RunJobRequest_Overrides{
			ContainerOverrides: []*runpb.RunJobRequest_Overrides_ContainerOverride{
				{
					Env: []*runpb.EnvVar{
						{Name: "RUN_ID", Values: &runpb.EnvVar_Value{Value: runID}},
						{Name: "LANGUAGE", Values: &runpb.EnvVar_Value{Value: language}},
					},
				},
			},
		},
	}
	operation, err := jobsClient.RunJob(ctx, req)
	if err != nil {
		return err
	}

	execution, err := operation.Wait(ctx)
	if err != nil {
		return err
	}

	if execution.GetSucceededCount() == 0 {
		return fmt.Errorf("the runner container failed")
	}

	return nil
}

func readResult(ctx context.Context, path string) (runResult, error) {
	reader, err := storageClient.Bucket(bucketName).Object(path).NewReader(ctx)

	if err != nil {
		return runResult{}, err
	}

	defer reader.Close()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return runResult{}, err
	}

	var res runResult
	err = json.Unmarshal(raw, &res)
	return res, err
}

func deleteFiles(paths ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, p := range paths {
		storageClient.Bucket(bucketName).Object(p).Delete(ctx)
	}
}

func execute(ctx context.Context, language string, code string) (runResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	runID := uuid.NewString()
	codePath := "runs/" + runID + "/code"
	resultPath := "runs/" + runID + "/result.json"

	defer deleteFiles(codePath, resultPath)

	if err := uploadCode(ctx, codePath, code); err != nil {
		return runResult{}, fmt.Errorf("upload code %w", err)
	}

	if err := startJob(ctx, runID, language); err != nil {
		return runResult{}, fmt.Errorf("run job: %w", err)
	}

	res, err := readResult(ctx, resultPath)
	if err != nil {
		return runResult{}, fmt.Errorf("read result : %w", err)
	}

	return res, nil

}

var languages = map[string]bool{"python": true, "node": true, "go": true}

type runRequest struct {
	Language string `json:"language"` // "python", "node" or "go" (default "python")
	Code     string `json:"code"`
}

func runHandler(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// 1. read the JSON body
		var in runRequest
		if err := c.Bind().Body(&in); err != nil || in.Code == "" {
			return fiber.NewError(fiber.StatusBadRequest, "code is required")
		}
		if in.Language == "" {
			in.Language = "python"
		}
		if !languages[in.Language] {
			return fiber.NewError(fiber.StatusBadRequest, "language must be python, node or go")
		}

		// 2. run the code in a brand-new container
		res, err := execute(c.Context(), in.Language, in.Code)
		if err != nil {
			log.Println("execute failed:", err) // the real reason goes to our logs only
			return fiber.NewError(fiber.StatusBadGateway, "could not run your code, try again")
		}

		// 3. save the result for this user
		sub := Submission{
			UserID:   fiber.Locals[uint](c, "userID"),
			Language: in.Language,
			Code:     in.Code,
			Stdout:   res.Stdout,
			Stderr:   res.Stderr,
			ExitCode: res.ExitCode,
			TimedOut: res.TimedOut,
		}
		if err := db.Create(&sub).Error; err != nil {
			return err
		}

		// 4. send it back
		return c.JSON(sub)
	}
}

func listSubmissions(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var subs []Submission
		err := db.Where("user_id = ?", fiber.Locals[uint](c, "userID")). // only YOUR rows
											Order("id desc").Limit(50).Find(&subs).Error
		if err != nil {
			return err
		}
		return c.JSON(subs)
	}
}
