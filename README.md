# Alktrace

The objective is to get an insight of the path taken even after NAT happening
(which obfuscates the path) done by Service implementation. 
Protocol & port must be customizable depending on the service definition.

If this is not possible, the second objective is to give all the infos of the 
pods/nodes/svc/etc aggregated in a nice to help understanding what's happening
after the NAT.

As an example, this trace to the service `4.5.3.2` : 

`alktrace -recurse -auto -proto tcp -p 443 4.5.3.2`

returns :
  * the path to the service using TCP/443
  * the infos about the Service (retrieved from the k8s API)
  * the path to all the pods behind the service using TCP/443

```bash
./alktrace -h

Usage: 
 sudo ./alktrace [options] <domain/ip> 
Options:
  -alsologtostderr
    	log to standard error as well as files
  -auto
    	Passing this flag instead of svcName to try to find automatically infos on the service traced
  -kconf string
    	Path to the kubeconfig. Defaults to ~/.kube/config (default "/home/christ/.kube/config")
  -log_backtrace_at value
    	when logging hits line file:N, emit a stack trace
  -log_dir string
    	If non-empty, write log files in this directory
  -logtostderr
    	log to standard error instead of files
  -ns string
    	Specify the namespace in which the service reside. Seeks into all namespaces by default
  -p string
    	Set the destination port to use. Using 80 by default (default "80")
  -proto string
    	Set the protocol to use. By default use udp on Linux. 
    	 Can be tcp, icmp
  -recurse
    	Tries to trace the pods found if there is any
  -stderrthreshold value
    	logs at or above this threshold go to stderr
  -svc string
    	Specify the service name for which you want more infos
  -v value
    	log level for V logs
  -vmodule value
    	comma-separated list of pattern=N settings for file-filtered logging
```

