// Alktrace: Tries to get as much infos as it can about a k8s service to help
// tbshooting networking issues
package main

import (
	"errors"
	"flag"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	typev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

func main() {
	// Handling User input
	flag.Usage = func() {
		fmt.Printf("Usage: \n sudo ./alktrace [options] <domain/ip> \nOptions:\n")
		flag.PrintDefaults()
	}
	protoToUse := flag.String("proto", "", "Set the protocol to use. By default use udp on Linux. \n Can be tcp, icmp")
	portDest := flag.String("p", "80", "Set the destination port to use. Using 80 by default")
	kconfig := flag.String("kconf", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "Path to the kubeconfig. Defaults to ~/.kube/config")
	namespace := flag.String("ns", "", "Specify the namespace in which the service reside. Seeks into all namespaces by default")
	svcName := flag.String("svc", "", "Specify the service name for which you want more infos")
	auto := flag.Bool("auto", false, "Passing this flag instead of svcName to try to find automatically infos on the service traced")
	recurse := flag.Bool("recurse", false, "Tries to trace the pods found if there is any")
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		return
	}
	host := dns(flag.Args()[0])
	proto := proto(*protoToUse)
	var wg sync.WaitGroup

	// Trace to the dest given
	trace(host, proto, *portDest, false, wg)

	if *svcName != "" || *auto {
		// Get all infos from k8s about the destination
		pods, err := getK8sInfos(*kconfig, host, *svcName, *namespace)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if *recurse {
			for _, pod := range pods.Items {
				fmt.Println("Tracing the pod : ", pod.Status.PodIP)
				wg.Add(1)
				go trace(pod.Status.PodIP, proto, *portDest, true, wg)
			}
			wg.Wait()
		}
	}
}

func dns(host string) string {
	addresses, err := net.LookupHost(host)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	return addresses[0]
}

func proto(proto string) string {
	switch proto {
	case "tcp":
		return "-T"
	case "icmp":
		return "-I"
	default:
		return "-U"
	}
}

func trace(host string, proto string, port string, pod bool, wg sync.WaitGroup) {
	if pod {
		defer wg.Done()
	}
	result, err := exec.Command("sudo", "traceroute", "-w", "1", "-q", "1", proto, "-p", port, host).Output()
	if err != nil {
		fmt.Println(err)
		fmt.Fprintln(os.Stdout, "Please check that you have traceroute (not the one from inetutils which is not powerful enough)")
		fmt.Fprintln(os.Stdout, "Please check also your firewall rules")
		return
	}
	fmt.Fprintf(os.Stdout, "%s", result)
}

func getK8sInfos(kconfig string, host string, svcName string, namespace string) (*corev1.PodList, error) {
	//fmt.Fprintf(os.Stdout, "Using config from : %s\n", kconfig)
	k8sClient, err := getClient(kconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var svc *corev1.Service

	if svcName != "" {
		svc, err = getServiceForDeployment(svcName, namespace, k8sClient)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
	} else {
		svc, err = findService(host, namespace, k8sClient)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
	}

	pods, err := getPodsForSvc(svc, namespace, k8sClient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	for _, pod := range pods.Items {
		fmt.Fprintf(os.Stdout, "    %v (%v) on Node : %v \n", pod.Name, pod.Status.PodIP, pod.Status.HostIP)
	}
	return pods, nil
}

func getClient(configLocation string) (typev1.CoreV1Interface, error) {
	kubeconfig := filepath.Clean(configLocation)
	//fmt.Fprintf(os.Stdout, "Cleaned location : %s\n", kubeconfig)
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Fprintf(os.Stdout, "Config built: %v\n", config)
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset.CoreV1(), nil
}

func getServiceForDeployment(deployment string, namespace string, k8sClient typev1.CoreV1Interface) (*corev1.Service, error) {
	listOptions := metav1.ListOptions{}
	svcs, err := k8sClient.Services(namespace).List(listOptions)
	if err != nil {
		log.Fatal(err)
	}
	for _, svc := range svcs.Items {
		if strings.Contains(svc.Name, deployment) {
			fmt.Fprintf(os.Stdout, "\n The service reached (%v) serves the pods : \n", svc.Name)
			return &svc, nil
		}
	}
	return nil, errors.New("cannot find service for deployment")
}

func findService(host string, namespace string, k8sClient typev1.CoreV1Interface) (*corev1.Service, error) {
	listOptions := metav1.ListOptions{}
	svcs, err := k8sClient.Services(namespace).List(listOptions)
	if err != nil {
		log.Fatal(err)
	}
	for _, svc := range svcs.Items {
		if svc.Spec.ClusterIP == host {
			fmt.Fprintf(os.Stdout, "\n The service reached (%v) serves the pods : \n", svc.Name)
			return &svc, nil
		}
	}
	return nil, errors.New("cannot find service for deployment")
}

func getPodsForSvc(svc *corev1.Service, namespace string, k8sClient typev1.CoreV1Interface) (*corev1.PodList, error) {
	set := labels.Set(svc.Spec.Selector)
	listOptions := metav1.ListOptions{LabelSelector: set.AsSelector().String()}
	pods, err := k8sClient.Pods(namespace).List(listOptions)
	return pods, err
}
