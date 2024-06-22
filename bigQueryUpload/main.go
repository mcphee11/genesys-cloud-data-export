package bigqueryupload

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"cloud.google.com/go/bigquery"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/googleapis/google-cloudevents-go/cloud/storagedata"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	functions.CloudEvent("storage", start)
}

func start(ctx context.Context, e event.Event) error {
	log.Printf("Event ID: %s", e.ID())
	log.Printf("Event Type: %s", e.Type())

	// Get and check for variables
	projectId := os.Getenv("PROJECTID")
	if projectId == "" {
		return fmt.Errorf("PROJECTID not set")
	}
	datasetId := os.Getenv("DATASETID")
	if datasetId == "" {
		return fmt.Errorf("DATASETID not set")
	}
	tableIdConversation := os.Getenv("TABLEID_CONVERSATION")
	if tableIdConversation == "" {
		return fmt.Errorf("TABLEID_CONVERSATION not set")
	}
	tableIdUsers := os.Getenv("TABLEID_USERS")
	if tableIdUsers == "" {
		return fmt.Errorf("TABLEID_USERS not set")
	}
	tableIdQueues := os.Getenv("TABLEID_QUEUES")
	if tableIdQueues == "" {
		return fmt.Errorf("TABLEID_QUEUES not set")
	}

	var data storagedata.StorageObjectData
	if err := protojson.Unmarshal(e.Data(), &data); err != nil {
		return fmt.Errorf("protojson.Unmarshal: %w", err)
	}

	log.Printf("Bucket: %s", data.GetBucket())
	log.Printf("File: %s", data.GetName())
	location := data.GetBucket() + "/" + data.GetName()
	fmt.Printf("location: %v\n", location)

	// Check for file type to upload to correct table
	if strings.Contains(location, "conversation") {
		err := importJSONExplicitSchema(projectId, datasetId, tableIdConversation, location)
		if err != nil {
			return fmt.Errorf("error uploading to BigQuery conversation: %v", err)
		}
		fmt.Println("success uploaded conversation")
	}

	if strings.Contains(location, "users") {
		err := importJSONExplicitSchema(projectId, datasetId, tableIdUsers, location)
		if err != nil {
			return fmt.Errorf("error uploading to BigQuery users: %v", err)
		}
		fmt.Println("success uploaded users")
	}

	if strings.Contains(location, "queues") {
		err := importJSONExplicitSchema(projectId, datasetId, tableIdQueues, location)
		if err != nil {
			return fmt.Errorf("error uploading to BigQuery queues: %v", err)
		}
		fmt.Println("success uploaded queues")
	}

	return nil
}

func importJSONExplicitSchema(projectID, datasetID, tableID, location string) error {
	ctx := context.Background()
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return fmt.Errorf("bigquery.NewClient: %v", err)
	}
	defer client.Close()

	var loader *bigquery.Loader
	gcsRef := bigquery.NewGCSReference("gs://" + location)
	gcsRef.SourceFormat = bigquery.JSON

	// Check for location type to use correct Write mode
	if strings.Contains(location, "conversation") {
		gcsRef.Schema = createConversationDetailsSchema()
		loader = client.Dataset(datasetID).Table(tableID).LoaderFrom(gcsRef)
		loader.WriteDisposition = bigquery.WriteAppend
	}

	if strings.Contains(location, "users") {
		gcsRef.AutoDetect = true
		loader = client.Dataset(datasetID).Table(tableID).LoaderFrom(gcsRef)
		loader.WriteDisposition = bigquery.WriteTruncate
	}

	if strings.Contains(location, "queues") {
		gcsRef.AutoDetect = true
		loader = client.Dataset(datasetID).Table(tableID).LoaderFrom(gcsRef)
		loader.WriteDisposition = bigquery.WriteTruncate
	}

	job, err := loader.Run(ctx)
	if err != nil {
		return err
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return err
	}

	if status.Err() != nil {
		if strings.Contains(status.Errors[0].Message, "Provided Schema does not match Table") {
			fmt.Printf("Missing schema: %v", status.Errors[0].Message)
			startIndex := strings.LastIndex(status.Errors[0].Message, ":")
			endIndex := strings.LastIndex(status.Errors[0].Message, ")")
			missing := status.Errors[0].Message[startIndex+1 : endIndex]
			fmt.Printf("Missing component: %v", missing)
		}
		return fmt.Errorf("job completed with errors: %v", status.Errors)
	}
	return nil
}
