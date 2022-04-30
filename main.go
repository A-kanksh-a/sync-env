package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	homeDir := os.Getenv("HOME")
	kubeconfigFile := homeDir + "/.kube/config"
	kubeconfig := flag.String("kubeconfig", kubeconfigFile, "Kubeconfig File location")
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		// handle error
		fmt.Printf("erorr %s building config from flags\n", err.Error())
		config, err = rest.InClusterConfig()
		if err != nil {
			fmt.Printf("error %s, getting inclusterconfig", err.Error())
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		// handle error
		fmt.Printf("error %s, creating clientset\n", err.Error())
	}
	fmt.Println("List all Deployments in default Namespace")

	deployments, err := clientset.AppsV1().Deployments("default").List(context.Background(), v1.ListOptions{})
	if err != nil {
		// handle error
		fmt.Printf("error %s, listing deployments\n", err.Error())
	}

	for _, deployment := range deployments.Items {
		fmt.Println(deployment.Name)
	}
	fmt.Println("List all Configmap in default Namespace")
	cmList, err := clientset.CoreV1().ConfigMaps("default").List(context.Background(), v1.ListOptions{})

	if err != nil {
		// handle error
		fmt.Printf("error %s, listing configMaps\n", err.Error())
	}

	for _, cm := range cmList.Items {
		fmt.Println(cm.Name)
	}
	//If we want to watch specific resources
	labelOptions := informers.WithTweakListOptions(func(opts *v1.ListOptions) {
		//opts.LabelSelector = "app=nats-box"
	})

	// By default NewSharedInformerFactory creates informerfactory for all Namespaces
	// Use NewSharedInformerFactoryWithOptions for creating informer instance in specific namespace
	informers := informers.NewSharedInformerFactoryWithOptions(clientset, 10*time.Minute, informers.WithNamespace("default"), labelOptions)
	ch := make(chan struct{})
	c := newController(clientset, informers.Core().V1().ConfigMaps())
	informers.Start(ch)
	c.run(ch)
	if err != nil {
		// handle error
		fmt.Printf("error %s, listing informers\n", err.Error())
	}
	fmt.Println(informers)
}
