#KubeJobWatcher

Kubernetes does not automatically clean up jobs.This can lead to huge numbers of failed job pods building up and bringing your system to a halt.

Enter KubeJobWatcher.

KubeJobWatcher cleans up jobs. Every 10 seconds KubeJobWatcher will query Kube for a list of jobs. If those jobs have greater than 1 failure or success the job will be removed along with any associated pods.

## To use

```
docker build -t your_org/kubejobwatcher .
docker push your_org/kubejobwatcher
```

or use the public version hosted by Drud at `drud/kubejobwatcher`.

Then use the deployment manifest found in kube/exampledep.yaml to create a deployment for yourself.

then run:

```
kubectl create -f yourdeployment.yaml
```

## Config options

To print job logs edit main.go and change the constant `printLogs` to `true`
