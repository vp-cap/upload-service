package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	database "github.com/vp-cap/data-lib/database"
	storage "github.com/vp-cap/data-lib/storage"
	config "github.com/vp-cap/upload-service/config"
	pb "github.com/vp-cap/upload-service/genproto"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	"google.golang.org/protobuf/proto"
	"github.com/streadway/amqp"
)

const (
	// UploadSizeLimit of 10 MB
	UploadSizeLimit = 10 << 20 
	// TempDir to store video uploads
	TempDir = "F:/go/src/vp-cap/upload-service/temp-uploads"
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
	conn, err := amqp.Dial(configs.Services.RabbitMq)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Println(err)
		return
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"task_queue", // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Println(err)
		return
	}

	task := &pb.Task{VideoName: name, VideoCid: cid}
	body, err := proto.Marshal(task)
	if err != nil {
		log.Fatalln("Failed to encode :", err)
	}
	err = ch.Publish(
		"",           // exchange
		q.Name,       // routing key
		false,        // mandatory
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/plain",
			Body:         []byte(body),
		})
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Video:", cid, "submitted to task queue")
}

// HTTP handle video upload
func uploadVideo(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

// HTTP handle ad upload
func uploadAdvertisement(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	switch r.Method {
	case "POST":
		ctx := r.Context()
		r.ParseMultipartForm(UploadSizeLimit)

		// get information from file
		name := r.FormValue("adName")
		imageLink := r.FormValue("imageLink")
		redirectURL := r.FormValue("redirectUrl")
		object := r.FormValue("object")

		log.Println("DB Entry")
		// make an entry in the database
		err := db.InsertAd(ctx, database.Advertisement{
			Name: name,
			ImageLink: imageLink,
			Object: object,
			RedirectURL: redirectURL,
		})
		if (err != nil) {
			fmt.Fprintf(w, "Ad Upload Failed")
			log.Println(err)
			return
		}
		log.Println("Ad Upload Successful")
		fmt.Fprintf(w, "Ad Upload Successful")

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

	router := httprouter.New()
	router.POST("/video", uploadVideo)
	router.POST("/ad", uploadAdvertisement)

	handler := cors.Default().Handler(router)

	log.Println("Serving on", configs.Server.Port)
	http.ListenAndServe(":" + configs.Server.Port, handler)
}