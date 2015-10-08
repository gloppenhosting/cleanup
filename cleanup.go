package main //import "github.com/tutumcloud/cleanup"

import (
	"flag"
	"log"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

var (
	pDockerHost         = flag.String("dockerHost", "unix:///var/run/docker.sock", "docker host")
	pImageCleanInterval = flag.Int("imageCleanInterval", 1, "interval to run image cleanup")
	pImageCleanDelayed  = flag.Int("imageCleanDelayed", 1800, "delayed time to clean the images")
	pImageLocked        = flag.String("imageLocked", "", "images to avoid being cleaned")
)

func main() {
	flag.Parse()

	var apiVersion string
	client, err := getDockerClient(*pDockerHost)
	if err != nil {
		log.Fatalf("Docker %s:%s", err, *pDockerHost)
	}

	version, err := client.Version()
	if err == nil {
		log.Print("Docker version: ", version.Get("Version"))
		apiVersion = version.Get("ApiVersion")
		log.Print("Api version: ", apiVersion)
	} else {
		log.Fatalf("Faid to get docker version: %s", err)
	}

	cleanImages(client)
}

func getDockerClient(host string) (*docker.Client, error) {
	client, err := docker.NewClient(host)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func cleanImages(client *docker.Client) {
	log.Printf("Img Cleanup: the following images will be locked: %s", *pImageLocked)
	log.Println("Img Cleanup: starting image cleanup ...")
	for {
		// imageIdMap[imageID] = isRemovable
		imageIdMap := make(map[string]bool)

		// Get the image ID list before the cleanup
		images, err := client.ListImages(docker.ListImagesOptions{All: false})
		if err != nil {
			log.Println("Img Cleanup: cannot get images list", err)
			time.Sleep(time.Duration(*pImageCleanInterval+*pImageCleanDelayed) * time.Second)
			continue
		}

		for _, image := range images {
			imageIdMap[image.ID] = true
		}

		// Get the image IDs used by all the containers
		containers, err := client.ListContainers(docker.ListContainersOptions{All: true})
		if err != nil {
			log.Println("Img Cleanup: cannot get container list", err)
			time.Sleep(time.Duration(*pImageCleanInterval+*pImageCleanDelayed) * time.Second)
			continue
		} else {
			inspect_error := false
			for _, container := range containers {
				containerInspect, err := client.InspectContainer(container.ID)
				if err != nil {
					inspect_error = true
					log.Println("Img Cleanup: cannot get container inspect", err)
					break
				}
				delete(imageIdMap, containerInspect.Image)
			}
			if inspect_error {
				time.Sleep(time.Duration(*pImageCleanInterval+*pImageCleanDelayed) * time.Second)
				continue
			}
		}

		// Get all the locked image ID
		if *pImageLocked != "" {
			lockedImages := strings.Split(*pImageLocked, ",")
			for _, lockedImage := range lockedImages {
				imageInspect, err := client.InspectImage(strings.Trim(lockedImage, " "))
				if err == nil {
					delete(imageIdMap, imageInspect.ID)
				}

			}
		}

		// Sleep for the delay time
		log.Printf("Img Cleanup: wait %d seconds for the cleaning", *pImageCleanDelayed)
		time.Sleep(time.Duration(*pImageCleanDelayed) * time.Second)

		// Get the image IDs used by all the containers again after the delay time
		containersDelayed, err := client.ListContainers(docker.ListContainersOptions{All: true})
		if err != nil {
			log.Println("Img Cleanup: cannot get container list", err)
			time.Sleep(time.Duration(*pImageCleanInterval) * time.Second)
			continue
		} else {
			inspect_error := false
			for _, container := range containersDelayed {
				containerInspect, err := client.InspectContainer(container.ID)
				if err != nil {
					inspect_error = true
					log.Println("Img Cleanup: cannot get container inspect", err)
					break
				}
				delete(imageIdMap, containerInspect.Image)
			}
			if inspect_error {
				time.Sleep(time.Duration(*pImageCleanInterval) * time.Second)
				continue
			}
		}

		// Remove the unused images
		counter := 0
		for id, removable := range imageIdMap {
			if removable {
				log.Printf("Img Cleanup: removing image %s", id)
				err := client.RemoveImage(id)
				if err != nil {
					log.Printf("Img Cleanup: %s", err)
				}
				counter += 1
			}
		}
		log.Printf("Img Cleanup: %d images have been removed", counter)

		// Sleep again
		log.Printf("Img Cleanup: next cleanup will be start in %d seconds", *pImageCleanInterval)
		time.Sleep(time.Duration(*pImageCleanInterval) * time.Second)
	}
}
