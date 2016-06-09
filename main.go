package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	tokenPath         = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	pollInterval      = 10 // in seconds
	succeedThreshhold = 1  // number of successes before cleanup
	failThreshhold    = 1  // number of fails before cleanup
	printLogs         = false
)

var kubeURL string
var kubeToken string

type jobList struct {
	APIVersion string `json:"apiVersion"`
	Items      []struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			CreationTimestamp string `json:"creationTimestamp"`
			Labels            struct {
				Controller_uid string `json:"controller-uid"`
				Job_name       string `json:"job-name"`
			} `json:"labels"`
			Name            string `json:"name"`
			Namespace       string `json:"namespace"`
			ResourceVersion string `json:"resourceVersion"`
			SelfLink        string `json:"selfLink"`
			UID             string `json:"uid"`
		} `json:"metadata"`
		Status struct {
			CompletionTime string `json:"completionTime"`
			Conditions     []struct {
				LastProbeTime      string `json:"lastProbeTime"`
				LastTransitionTime string `json:"lastTransitionTime"`
				Status             string `json:"status"`
				Type               string `json:"type"`
			} `json:"conditions"`
			StartTime string `json:"startTime"`
			Succeeded int    `json:"succeeded"`
			Failed    int    `json:"failed"`
			Active    int    `json:"active"`
		} `json:"status"`
	} `json:"items"`
	Kind     string   `json:"kind"`
	Metadata struct{} `json:"metadata"`
}

type podList struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Metadata   struct {
	} `json:"metadata"`
	Items []struct {
		Kind       string `json:"kind"`
		APIVersion string `json:"apiVersion"`
		Metadata   struct {
			Name              string    `json:"name"`
			GenerateName      string    `json:"generateName"`
			Namespace         string    `json:"namespace"`
			SelfLink          string    `json:"selfLink"`
			UID               string    `json:"uid"`
			ResourceVersion   string    `json:"resourceVersion"`
			CreationTimestamp time.Time `json:"creationTimestamp"`
			Labels            struct {
				ControllerUID string `json:"controller-uid"`
				JobName       string `json:"job-name"`
			} `json:"labels"`
			Annotations struct {
				KubernetesIoCreatedBy string `json:"kubernetes.io/created-by"`
			} `json:"annotations"`
		} `json:"metadata"`
		Status struct {
			Phase      string `json:"phase"`
			Conditions []struct {
				Type               string      `json:"type"`
				Status             string      `json:"status"`
				LastProbeTime      interface{} `json:"lastProbeTime"`
				LastTransitionTime time.Time   `json:"lastTransitionTime"`
				Reason             string      `json:"reason"`
			} `json:"conditions"`
			HostIP            string    `json:"hostIP"`
			PodIP             string    `json:"podIP"`
			StartTime         time.Time `json:"startTime"`
			ContainerStatuses []struct {
				Name  string `json:"name"`
				State struct {
					Terminated struct {
						ExitCode    int       `json:"exitCode"`
						Reason      string    `json:"reason"`
						StartedAt   time.Time `json:"startedAt"`
						FinishedAt  time.Time `json:"finishedAt"`
						ContainerID string    `json:"containerID"`
					} `json:"terminated"`
				} `json:"state"`
				LastState struct {
				} `json:"lastState"`
				Ready        bool   `json:"ready"`
				RestartCount int    `json:"restartCount"`
				Image        string `json:"image"`
				ImageID      string `json:"imageID"`
				ContainerID  string `json:"containerID"`
			} `json:"containerStatuses"`
		} `json:"status"`
	} `json:"items"`
}

func kubectl(command string, token string, url string, namespace string, tojson bool) ([]byte, error) {
	fullCmd := fmt.Sprintf(
		"%s --token=%s --server=%s --insecure-skip-tls-verify=true",
		command,
		token,
		url,
	)
	if namespace == "" {
		fullCmd += " --all-namespaces"
	} else {
		fullCmd += " --namespace=" + namespace
	}
	if tojson {
		fullCmd += " -o json"
	}

	cmdParts := strings.Fields(fullCmd)
	out, err := exec.Command("/bin/kubectl", cmdParts...).Output()

	return out, err
}

func getPods(selector string, namespace string) (podList, error) {
	var pods podList
	podBytes, err := kubectl(
		fmt.Sprintf("get pods -a --selector=%s", selector),
		kubeToken,
		kubeURL,
		namespace,
		true,
	)
	if err != nil {
		return pods, err
	}

	err = json.Unmarshal(podBytes, &pods)
	if err != nil {
		return pods, err
	}

	return pods, nil

}

func getLogs(jobNAme string, namespace string) (string, error) {
	log.Println("getting logs")
	pods, err := getPods(fmt.Sprintf("job-name=%s", jobNAme), namespace)
	if err != nil {
		return "", err
	}

	logBytes, err := kubectl(
		fmt.Sprintf("logs %s", pods.Items[0].Metadata.Name),
		kubeToken,
		kubeURL,
		namespace,
		false,
	)
	if err != nil {
		return "", err
	}

	return string(logBytes), nil
}

func deleteJob(jobName string, namespace string) error {
	log.Println("deleting job:", jobName)
	_, err := kubectl(
		fmt.Sprintf("delete job %s", jobName),
		kubeToken,
		kubeURL,
		namespace,
		false,
	)
	if err != nil {
		return err
	}

	return nil
}

func main() {

	kubeHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	kubePort := os.Getenv("KUBERNETES_SERVICE_PORT")
	if kubeHost == "" || kubePort == "" {
		log.Fatalln("KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be set.")
	}
	kubeURL = fmt.Sprintf("https://%s:%s", kubeHost, kubePort)

	tokenBytes, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		log.Fatalln("issue while reading token file:\n", err)
	}
	kubeToken = string(tokenBytes)

	for {
		jobsJSON, err := kubectl("get jobs", string(tokenBytes), kubeURL, "", true)
		if err != nil {
			log.Fatalln(err)
		}

		var mylist jobList
		err = json.Unmarshal(jobsJSON, &mylist)
		if err != nil {
			log.Fatal(err)
		}

		for _, v := range mylist.Items {
			log.Printf("job: %s, succeeded:%d, failed:%d, active:%d",
				v.Metadata.Name,
				v.Status.Succeeded,
				v.Status.Failed,
				v.Status.Active,
			)

			var cleanup bool
			switch {
			case v.Status.Succeeded >= succeedThreshhold:
				cleanup = true
			case v.Status.Failed >= failThreshhold:
				cleanup = true
			default:
				cleanup = false
			}

			if cleanup {

				// do your custom cleanup here

				// optionally print the job logs
				if printLogs {
					logs, err := getLogs(v.Metadata.Name, v.Metadata.Namespace)
					if err != nil {
						log.Fatalln(err)
					}

					log.Println(logs)
				}

				err = deleteJob(v.Metadata.Name, v.Metadata.Namespace)
				if err != nil {
					log.Fatal(err)
				}
			}
		}

		//if len(mylist.Items) == 0 {
		//	log.Println("No jobs ready for cleanup. sleeping 10 seconds...")
		//}

		time.Sleep(pollInterval * time.Second)
	}
}
