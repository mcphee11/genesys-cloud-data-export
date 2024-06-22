# genesys-cloud-data-export

An example of exporting analytics data into Google Cloud Big Query for historical reporting usage. For this example I will be using Google Cloud services pricing is dependent on that provider so be aware of their costs and data usage tiers etc.

`This example requires you to already have experience with both Google Cloud, Genesys Cloud & Golang programming language`

For reference I have done some example pricing based on 10Gb storage and what I would consider normal API usage. BEWARE your costs may differ...

![](/docs/images/solution.png?raw=true)

From here you can use a BI Tool in my case im using Google Looker Studio, to build out dashboards on the data you require.

![](/docs/images/report.png?raw=true)

## Step 1 - Create OAuth

The first thing you need to do is to create a `client credentials` OAuth token so that the code can access the Genesys Cloud APIs to retrieve the data.

![](/docs/images/oauth.png?raw=true)

For the `roles` this will depend on the APIs you are wanting to include in your report. For this example you will at a minimum need the below scopes:

```
analytics
users
routing
```

Ensure you copy the details once saved as you will need these later on.

```
clientId
client secret
```

## Step 2 - GCP Project

Ensure you have a Google Cloud Project you plan to use for this that has a billing account linked and enabled Cloud Functions, BigQuery, Storage, Pub/Sub & Arc. You will also need to have the [GCloud CLI](https://cloud.google.com/sdk/docs/install) installed and setup to run the install commands. If any of these are NOT installed or setup correctly you will see errors from the CLI stating so and how to fix them including links to the Google documentation.

Also make sure you have [Golang](https://go.dev/doc/install) installed and setup as well, version 1.22+.

- Create a GCP `Bucket` created under your project and copy down the name of this bucket as you will need it later.

- Create a BigQuery `Dataset` and copy down that name as well.

![](/docs/images/dataset.png?raw=true)

## Step 3 - Deploy Cloud Functions

There are 2x Google Cloud functions that need to be deployed. Both are located in this repo and are in separate folders. To make my life easier I have created and use separate bash shell files to run the commands and have these inside the `tasks.json` file for VSCode but have ensured that the .sh files themselves are in the .gitignore file for security. For simplicity though you can run these commands manually inside your terminal.

As im based in Australia I am deploying the function into the `australia-southeast1` region. You can change this to be a different region based on where you are.

### getDataQueries

Ensure you are inside the ./getDataQueries dir then run the below command with updating the parameters to be based on your environment.

```
gcloud functions deploy getDataQueries --gen2 --runtime=go122 --region=australia-southeast1 --source=. --entry-point=genesysData --trigger-topic=genesysData --set-env-vars REGION=ENTER_YOUR_REGION,CLIENTID=ENTER_YOUR_CLIENTID,SECRET=ENTER_YOUR_SECRET,BUCKETNAME=ENTER_YOUR_BUCKET_NAME,PROJECTID=ENTER_YOUR_GOOGLE_PROJECTID,DATASETID=ENTER_YOUR_DATASET_NAME,TABLEID_CONVERSATION=conversations
```

For example my `REGION` is `mypurecloud.com.au` as im using an ORG in the APSE2 region of AWS.

This will deploy the first Function.

### bigQueryUpload

Ensure you are inside the ./bigQueryUpload dir then run the below command with updating the parameters to be based on your environment.

```
gcloud functions deploy bigQueryUpload --gen2 --runtime=go122 --region=australia-southeast1 --source=. --entry-point=storage --trigger-event-filters="type=google.cloud.storage.object.v1.finalized" --trigger-event-filters="bucket=ENTER_YOUR_BUCKET_NAME" --set-env-vars PROJECTID=ENTER_YOUR_GOOGLE_PROJECTID,DATASETID=ENTER_YOUR_DATASET_NAME,TABLEID_CONVERSATION=conversations,TABLEID_USERS=users,TABLEID_QUEUES=queues
```

This will then deploy the second function.

## Step 4 - Time based trigger

Now that all the pieces are in place you just need to create a PUB/SUB topic then a Schedule to trigger the first function based on the time. In my case I have it trigger at Midnight each night to GET the data and populate the tables.

Create a Topic called `genesysData` then a schedule to trigger when you need, In my case I have made it at 00:00 each night this uses the `unix-cron` [format](https://cloud.google.com/scheduler/docs/configuring/cron-job-schedules).

```
0 0 * * *
```

![](/docs/images/schedual.png?raw=true)

Ensure that the execution is then pointing to the Topic that you created earlier.

Once saved you can "force run" the schedule to test if its working. When this first runs the code checks to see if there are any existing `conversations` table... if there is NOT it will not only create it but also GET a last months worth of data in the first run... if the table DOES exist then it will GET the last 24hours.

## Step 5 - Review the data

Open Up BiqQuery and look inside the `conversations` table as well as the `users` and `queues` tables and you should see data as well as be able to "preview" the data... If the tables DON'T exist then the code had an error running and you will need to look in the "Cloud Function" logs to see what went wrong.

# Step 6 - Building the Report

Open up Google [Looker Studio](https://lookerstudio.google.com/navigation/reporting) Create a new `Blank Report` then add a DataSource. By default a new report should open up the default selection options including `BigQuery` where we have already put out data.

![](/docs/images/sources.png?raw=true)

Once you have added all three data tables as sources. Then you will need to create the "Joins" of them to make the relationship between tables.

![](/docs/images/joins.png?raw=true)

From here I have then created a default dashboard as an example but you can then create and visualize the data how ever you wish.
