package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"log"
	"net/http"
	"path/filepath"
	"time"
	"github.com/filecoin-project/bacalhau/pkg/publicapi"
	"github.com/filecoin-project/bacalhau/pkg/model"
	"github.com/filecoin-project/bacalhau/pkg/ipfs"
	"github.com/filecoin-project/bacalhau/pkg/system"
)

func computeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/compute" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method is not supported.", http.StatusNotFound)
		return
	}

	r.ParseForm()
	x, y := r.Form["x"], r.Form["y"]

	fmt.Printf("x: %s, y: %s\n", x, y)

	cm := system.NewCleanupManager()
	defer cm.Cleanup()

	system.InitConfig()
	client := publicapi.NewAPIClient("http://"+os.Getenv("BACALHAU_API_HOST")+":"+os.Getenv("BACALHAU_API_PORT"))
	job, err := model.NewJobWithSaneProductionDefaults()
	if err != nil {
		fmt.Println(fmt.Errorf("unable to create job: %w", err))
		return
	}
	job.Spec.Docker.Image = "ubuntu"
	job.Spec.Docker.Entrypoint = []string{"echo", fmt.Sprintf("$((%s + %s))", x[1], y[1])}
	submittedJob, err := client.Submit(context.Background(), job, nil)
	if err != nil {
		fmt.Println(fmt.Errorf("error on submission: %w", err))
		return
	}

	time.Sleep(time.Second * 15)
	
	results, err := client.GetResults(context.Background(), submittedJob.ID)
	if err != nil {
		fmt.Println(fmt.Errorf("could not get results: %w", err))
		return
	}
	const dir = "tmp"
	downloadSettings := ipfs.IPFSDownloadSettings{
		TimeoutSecs:    int(ipfs.DefaultIPFSTimeout.Seconds()),
		OutputDir:      dir,
	}
	if err := ipfs.DownloadJob(context.Background(), cm, submittedJob, results, downloadSettings); err != nil {
		fmt.Println(fmt.Errorf("could not download job: %w", err))
		// don't return: not always fatal
	}

	content, err := ioutil.ReadFile(filepath.Join(dir, "stdout"))
	if err != nil {
		fmt.Println(fmt.Errorf("could not read file: %w", err))
		// don't return: not always fatal
	}
	fmt.Printf("Results: %s\n", content)

	os.RemoveAll(dir)
	os.Mkdir(dir, os.ModeDir)
}

func main() {
	fileServer := http.FileServer(http.Dir("./static")) // New code
	http.Handle("/", fileServer) // New code
	http.HandleFunc("/compute", computeHandler)


	fmt.Printf("Starting server at port 8080\n")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
