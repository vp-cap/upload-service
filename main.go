package main

import (
	"fmt"
	"context"
	"log"
	"net/http"
	"io/ioutil"
	"os"

	"google.golang.org/grpc"
	config "cap/upload-service/config"
	database "cap/data-lib/database"
	storage "cap/data-lib/storage"
	pb "cap/upload-service/genproto"
)

const (
	// UploadSizeLimit of 10 MB
	UploadSizeLimit = 10 << 20 
	// TempDir to store video uploads
	TempDir = "F:/go/src/cap/upload-service/temp-uploads"
	// TempFilePrefix for uploaded videos
	TempFilePrefix = "upload-*"
)

var (
	configs config.Configurations
	db database.Database = nil
	store storage.Storage = nil
)

// Submit task to the TaskAllocator
func submitTask (name string, cid string) {
	conn, err := grpc.Dial(configs.Services.TaskAllocator, grpc.WithInsecure())
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	client := pb.NewTaskInitServiceClient(conn)

	_, err = client.SubmitTask(context.Background(), &pb.Task{VideoName: name, VideoCid: cid})
	if err != nil {
		log.Println(err)
	}
	log.Println("Video:", cid, "submitted to task allocator")

}

// HTTP handle video upload
func uploadVideo(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		ctx := r.Context()
		r.ParseMultipartForm(UploadSizeLimit)

		// get information from file
		name := r.FormValue("videoName")
		description := r.FormValue("videoDesc")
		file, _, err := r.FormFile("videoFile")
		if (err != nil) {
			log.Println("Unable to retrieve file")
			log.Println(err)
			fmt.Fprintf(w, "Video Upload Failed")
			return
		}
		defer file.Close()

		// store temp file
		tempFile, err := ioutil.TempFile("", TempFilePrefix)
		if err != nil {
			log.Println(err)
			fmt.Fprintf(w, "Video Upload Failed")
			return
		}
		path := tempFile.Name()
		log.Println("Path: ", path)
		defer os.Remove(path)

		// read all of the contents of our uploaded file into a
		// byte array
		fileBytes, err := ioutil.ReadAll(file)
		if err != nil {
			log.Println(err)
			fmt.Fprintf(w, "Video Upload Failed")
			return
		}
		log.Println("Writing to temp")
		// write this byte array to our temporary file
		tempFile.Write(fileBytes)

		// store file in storage
		var cidString string
		log.Println("Uploading")
		cidString, err = store.UploadVideo(ctx, path)
		if (err != nil) {
			fmt.Fprintf(w, "Video Upload Failed")
			log.Println(err)
			return
		}
		log.Println("DB Entry")
		// make an entry in the database
		err = db.InsertVideo(ctx, database.Video{
			Name: name,
			Description: description,
			StorageLink: cidString,
		})
		if (err != nil) {
			fmt.Fprintf(w, "Video Upload Failed")
			log.Println(err)
			return
		}

		// submit task to allocator
		go submitTask(name, cidString)

		log.Println("Video Upload Successful")
		fmt.Fprintf(w, "Video Upload Successful")

	default:
		fmt.Fprintf(w, "Only POST method supported")
	}
}

func init() {
	var err error
	configs, err = config.GetConfigs()
	if err != nil {
		log.Println("Unable to get config")
	}
}

func main() {
	// Enable line numbers in logging
	log.SetFlags(log.LstdFlags | log.Lshortfile )
	log.Println(configs.Storage)

	ctx := context.Background()
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	// DB and store
	var err error
	db, err = database.GetDatabaseClient(ctx, configs.Database)
	if err != nil {
		log.Fatalln(err)
	}
	store, err = storage.GetStorageClient(configs.Storage)
	if err != nil {
		log.Fatalln(err)
	}

	http.HandleFunc("/", uploadVideo)
	log.Println("Serving on", configs.Server.Port)
	http.ListenAndServe(":" + configs.Server.Port, nil)
}