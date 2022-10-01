# sync-env

## Kubernetes Custom Controller

This controller updates a deployment env with ConfigMap reference whenever the configMap is created in the `default` namespace.  

## On Local Environment

### build the binary

```
go build
```

### Execute the binary

```
./sync-env
```

## On K8S Cluster

### Build the controller image and push to docker registry 
```
docker build -t akankshakumari393/sync-env:0.0.1 .
docker push akankshakumari393/sync-env:0.0.1
```

### create a namespace in which controller would run

```
kubectl create namespace sync-env
```

### Create Role in default namespace to give permission for configmaps, deployments 

```
kubectl create role default-role --verb=update,get,list,watch --resource=deployments,configmaps
# kubectl create -f k8s-resources/role.yaml
```

### Create RoleBindings in default namespace to give access to Service account mapped with controller 

```
kubectl create rolebinding default-role-binding --role=default-role --user=system:serviceaccount:sync-env:default
# kubectl create -f k8s-resources/rolebinding.yaml
```

Create deployment
kubectl create -f k8s-resources/deployment.yaml -n sync-env

### Now any configmap added to default namespace would be added to env of the deployment 
