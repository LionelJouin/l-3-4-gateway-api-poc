# Layer 3/4 Gateway API PoC

This is a PoC (Proof Of Concept) for layer 3 / 4 services using Gateway API (v1.1.0) over secondary networks in Kubernetes (v1.30).

Build
```
make generate
make
make generate-helm-chart
```

### pre-requisites:

Install Gateway API:
```
kubectl apply -k https://github.com/kubernetes-sigs/gateway-api/config/crd/experimental?ref=v1.1.0
```

Install Multus:
```
helm install multus ./deployments/Multus
```

## PoC 1: Service as Gateway API Route using [KPNG](https://github.com/kubernetes-sigs/kpng)

### Installation

Install the KPNG controller manager:
```
helm install poc _output/helm/l-3-4-gateway-api-poc-v0.0.0-latest.tgz
```

Install the example kpng Gateway/GatewayRouter/Service:
```
kubectl apply -f examples/kpng-gateway-api.yaml
```

Install example application behind the service:
```
helm install example-target-application-a ./examples/target-application/deployment/helm
```

Send traffic (400 connections to 20.0.0.1:4000)
```
docker exec -it vpn-a mconnect -address 20.0.0.1:4000 -nconn 400 -timeout 2s
```

### How does it work?

- The KPNG Controller Manager (kpng-cm) reconciles the Gateways of KPNG class by:
    1. Creating the daemonset corresponding to the Gateway.
    2. Finding all services that belong to the Gateway to:
        - Fetch all external IPs (VIPs) and add them to the Gateway status.
        - Fetch all pods selected by these services and create the corresponding endpointslices. Pods are added to the EndpointSlice only if an IP can be found. An IP can be found if the network status annotation contains one of the networks configured in the Gateway network annotation.
- The Router reconciles the Gateway by finding all GatewayRouters and fetching the addresses in the Gateway status to configure Bird accordingly.

![Flow](docs/resources/service-kpng.png)
