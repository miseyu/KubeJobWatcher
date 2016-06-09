#JobWatcher

Clean up jobs when finished. Every 10 seconds KubeJobWatcher will delete any job that has succeeded or failed.

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
