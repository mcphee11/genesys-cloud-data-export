package getdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/mypurecloud/platform-client-sdk-go/v129/platformclientv2"
	"google.golang.org/api/googleapi"
)

func init() {
	functions.CloudEvent("genesysData", start)
}

func start(ctx context.Context, e event.Event) error {
	// Get and check for variables
	region := os.Getenv("REGION")
	if region == "" {
		return fmt.Errorf("REGION not set")
	}
	clientId := os.Getenv("CLIENTID")
	if clientId == "" {
		return fmt.Errorf("CLIENTID not set")
	}
	secret := os.Getenv("SECRET")
	if secret == "" {
		return fmt.Errorf("SECRET not set")
	}
	bucketName := os.Getenv("BUCKETNAME")
	if bucketName == "" {
		return fmt.Errorf("BUCKETNAME not set")
	}
	// check for tableId to set first duration 3months
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

	config := platformclientv2.GetDefaultConfiguration()
	config.BasePath = "https://api." + region
	err := config.AuthorizeClientCredentials(clientId, secret)
	if err != nil {
		return fmt.Errorf("logging in error: %v", err)
	}
	fmt.Println("Logged In to Genesys Cloud")

	apiInstanceConversation := platformclientv2.NewConversationsApiWithConfig(config)
	apiInstanceUsers := platformclientv2.NewUsersApiWithConfig(config)
	apiInstanceQueues := platformclientv2.NewRoutingApiWithConfig(config)

	// Determine interval for last 7 days
	now := time.Now()
	var interval string

	// if no existing conversation table get 1 month worth to start
	existingTable := tableExists(projectId, datasetId, tableIdConversation)
	if existingTable {
		interval = fmt.Sprintf("%v/%v", now.AddDate(0, 0, -1).Format(time.RFC3339), now.Format(time.RFC3339))
	} else {
		interval = fmt.Sprintf("%v/%v", now.AddDate(0, -1, 0).Format(time.RFC3339), now.Format(time.RFC3339))
	}

	fmt.Printf("Interval Time: %v\n", interval)
	pageSizeCon := 100
	pageSize := 200
	format := now.Format("2006-01-02")
	users := []string{}

	dataCon, errCon := getData(*apiInstanceConversation, 1, pageSizeCon, interval)
	// Paging for conversations
	if errCon != nil {
		fmt.Printf("Error calling getData: %v\n", errCon)
	} else {
		fmt.Printf("Page Size: %v\n", *dataCon.TotalHits)
		if pageSizeCon < *dataCon.TotalHits {
			times := *dataCon.TotalHits / pageSizeCon
			fmt.Printf("paging... %v times.\n", times+1)
			//Get each page of data
			for i := 0; i < times+1; i++ {
				dataCon1, errCon1 := getData(*apiInstanceConversation, i+1, pageSizeCon, interval)
				if errCon1 != nil {
					return errCon1
				} else {
					payloadCon1, _ := json.Marshal(dataCon1)
					errCon2 := uploadToBucket(bucketName, format+"_conversation_"+strconv.Itoa(i)+".json", payloadCon1)
					if errCon2 != nil {
						fmt.Printf("Error Writing to bucket: %v\n", errCon2)
					}
				}
			}
		} else if pageSizeCon > *dataCon.TotalHits {
			payload, _ := json.Marshal(dataCon)
			err := uploadToBucket(bucketName, format+"_conversation_0.json", payload)
			if err != nil {
				fmt.Printf("Error Writing to bucket: %v\n", err)
			}
		}
	}

	dataQueue, errQueue := getQueues(*apiInstanceQueues, 1, pageSize)
	// Paging for queues
	if errQueue != nil {
		fmt.Printf("Error calling getQueues: %v\n", errQueue)
	} else {
		fmt.Printf("Page Size: %v\n", *dataQueue.Total)
		if pageSize < *dataQueue.Total {
			times := *dataQueue.Total / pageSize
			fmt.Printf("paging... %v times.\n", times)
			//Get each page of data
			for i := 0; i < times; i++ {
				dataQueue1, errQueue1 := getQueues(*apiInstanceQueues, i+1, pageSize)
				if errQueue1 != nil {
					return errQueue1
				} else {
					payload, _ := json.Marshal(dataQueue1)
					err := uploadToBucket(bucketName, format+"_queues_"+strconv.Itoa(i)+".json", payload)
					if err != nil {
						fmt.Printf("Error Writing to bucket: %v\n", err)
					}
				}
			}
		} else if pageSize > *dataQueue.Total {
			payload, _ := json.Marshal(dataQueue)
			err := uploadToBucket(bucketName, format+"_queues_0.json", payload)
			if err != nil {
				fmt.Printf("Error Writing to bucket: %v\n", err)
			}
		}
	}
	dataUser, errUser := getUsers(*apiInstanceUsers, users, 1, pageSize)
	// Paging for queues
	if errUser != nil {
		fmt.Printf("Error calling getUsers: %v\n", errUser)
	} else {
		fmt.Printf("Page Size: %v\n", *dataUser.Total)
		if pageSize < *dataUser.Total {
			times := *dataUser.Total / pageSize
			fmt.Printf("paging... %v times.\n", times)
			//Get each page of data
			for i := 0; i < times; i++ {
				dataUsers, errUsers := getUsers(*apiInstanceUsers, users, i+1, pageSize)
				if errUsers != nil {
					return errUsers
				} else {
					payload, _ := json.Marshal(dataUsers)
					err := uploadToBucket(bucketName, format+"_users_"+strconv.Itoa(i)+".json", payload)
					if err != nil {
						fmt.Printf("Error Writing to bucket: %v\n", err)
					}
				}
			}
		} else if pageSize > *dataUser.Total {
			payload, _ := json.Marshal(dataUser)
			err := uploadToBucket(bucketName, format+"_users_0.json", payload)
			if err != nil {
				fmt.Printf("Error Writing to bucket: %v\n", err)
			}
		}
	}
	return nil
}

// tableExists checks whether a table exists in a given dataset.
func tableExists(projectID, datasetID, tableID string) bool {
	ctx := context.Background()

	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return true
	}
	defer client.Close()

	tableRef := client.Dataset(datasetID).Table(tableID)
	if _, err = tableRef.Metadata(ctx); err != nil {
		if e, ok := err.(*googleapi.Error); ok {
			if e.Code == http.StatusNotFound {
				fmt.Println("dataset or table not found... GET 1 month.")
				return false
			}
		}
	}
	return true
}

func getData(apiInstanceConversation platformclientv2.ConversationsApi, pageNumber int, pageSizeCon int, interval string) (platformclientv2.Analyticsconversationqueryresponse, error) {
	body := platformclientv2.Conversationquery{
		Paging: &platformclientv2.Pagingspec{
			PageSize:   platformclientv2.Int(pageSizeCon),
			PageNumber: platformclientv2.Int(pageNumber),
		},
		Interval: &interval,
		OrderBy:  platformclientv2.String("conversationStart"),
		ConversationFilters: &[]platformclientv2.Conversationdetailqueryfilter{
			{
				VarType: platformclientv2.String("and"),
				Predicates: &[]platformclientv2.Conversationdetailquerypredicate{
					{
						VarType:  platformclientv2.String("metric"),
						Metric:   platformclientv2.String("tTalk"),
						Operator: platformclientv2.String("exists"),
					},
				},
			},
		},
	}
	// Query for conversation details asynchronously
	data, response, err := apiInstanceConversation.PostAnalyticsConversationsDetailsQuery(body)
	if err != nil {
		// this creates a panic when an err happens due to the *data return...
		return *data, err
	} else {
		fmt.Printf("Response:\n  Success: %v\n  Status code: %v\n  Correlation ID: %v\n", response.IsSuccess, response.StatusCode, response.CorrelationID)
		return *data, nil
	}
}

func getUsers(apiInstanceUsers platformclientv2.UsersApi, users []string, pageNumber int, pageSize int) (platformclientv2.Userentitylisting, error) {
	var jabberId []string                // A list of jabberIds to fetch by bulk (cannot be used with the \"id\" parameter)
	var sortOrder string                 // Ascending or descending sort order
	var expand []string                  // Which fields, if any, to expand
	var integrationPresenceSource string // Gets an integration presence for users instead of their defaults. This parameter will only be used when presence is provided as an \"expand\". When using this parameter the maximum number of users that can be returned is 100.
	var state string                     // Only list users of this state
	// Get Users
	data, response, err := apiInstanceUsers.GetUsers(pageSize, pageNumber, users, jabberId, sortOrder, expand, integrationPresenceSource, state)
	if err != nil {
		fmt.Printf("Error calling GetAnalyticsConversationsDetailsJobResults: %v\n", err)
		return *data, err
	} else {
		fmt.Printf("Response:\n  Success: %v\n  Status code: %v\n  Correlation ID: %v\n", response.IsSuccess, response.StatusCode, response.CorrelationID)
		return *data, nil
	}
}

func getQueues(apiInstanceQueues platformclientv2.RoutingApi, pageNumber int, pageSize int) (platformclientv2.Queueentitylisting, error) {
	var sortOrder string               // Note: results are sorted by name.
	var name string                    // Include only queues with the given name (leading and trailing asterisks allowed)
	var id []string                    // Include only queues with the specified ID(s)
	var divisionId []string            // Include only queues in the specified division ID(s)
	var peerId []string                // Include only queues with the specified peer ID(s)
	var cannedResponseLibraryId string // Include only queues explicitly associated with the specified canned response library ID
	var hasPeer bool                   // Include only queues with a peer ID

	data, response, err := apiInstanceQueues.GetRoutingQueues(pageNumber, pageSize, sortOrder, name, id, divisionId, peerId, cannedResponseLibraryId, hasPeer)
	if err != nil {
		fmt.Printf("Error calling GetRoutingQueues: %v\n", err)
		return *data, err
	} else {
		fmt.Printf("Response:\n  Success: %v\n  Status code: %v\n  Correlation ID: %v\n", response.IsSuccess, response.StatusCode, response.CorrelationID)
		return *data, nil
	}
}

func uploadToBucket(bucketName string, objectName string, payload []byte) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectName)

	//write to bucket
	writer := obj.NewWriter(ctx)
	_, err = writer.Write(payload)
	if err != nil {
		fmt.Printf("Writing Failed: %v\n", objectName)
		return err
	}
	fmt.Printf("Written Success: %v\n", objectName)
	err = writer.Close()
	if err != nil {
		fmt.Printf("Closing error on %v\n", objectName)
		return err
	}
	fmt.Printf("Closing %v\n", objectName)
	return nil
}
