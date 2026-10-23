# Deploying a cluster

A plan for running the cluster on Vultr, GCP and other clouds with as few manual steps as
possible. Everything after a one-time account setup is driven by CI/CD.

## Recommendation

Run the cluster on **managed Kubernetes on every cloud** (GKE on GCP, VKE on Vultr), from
**one container image and one Helm chart**, with infrastructure in **Terraform** and
deployments from **GitHub Actions**.

- Kubernetes gives each cloud the same primitives the cluster needs: stable node names
  (StatefulSet), discovery by DNS (headless Service), a TCP load balancer (Service of type
  `LoadBalancer`), readiness gating on `/healthz`, SIGTERM with a grace period for draining,
  and one-at-a-time rolling updates. The only per-cloud differences are a few Helm values
  and the Terraform module.
- Every change goes through the same pipeline: test, build the image once, deploy to
  staging, smoke test, promote the same image digest to production.
- What remains manual is a one-time setup per cloud (see [Manual steps](#manual-steps)).

Plain VMs (a GCE managed instance group, Vultr instances with cloud-init) also work, but
you have to build discovery, rolling restarts and health-gated deploys yourself. Serverless
container platforms (Cloud Run, App Runner) don't fit: they don't accept raw TCP or UDP
traffic, and they scale instances in and out freely.

## What the cluster needs from its environment

These follow from how the cluster works ([cluster.go](cluster.go), [registry.go](registry.go)):

| Need | Why | How on Kubernetes |
|---|---|---|
| Ports 1883/tcp (MQTT), 7946/tcp+udp (gossip), 7947/tcp (forwarding), 7948/tcp (raft), 8080/tcp (admin) | Each node listens on all of them | Container ports; only 1883 is exposed through the load balancer |
| A **private network** between nodes | Forwarding and raft links are plain TCP with no authentication; gossip is plaintext unless `memberlist.Config.SecretKey` is set | Pod network plus a NetworkPolicy that allows 7946–7948 only from other broker pods |
| Unique, stable node names | `NodeName` identifies raft voters | StatefulSet pod name (`mqtt-0`, `mqtt-1`, …) |
| Seeds | Joining needs at least one existing node | Headless Service DNS name, `mqtt-peers:7946`. memberlist resolves a hostname to all of its addresses and tries each one |
| Exactly one bootstrapping node | `Bootstrap` creates a new cluster only when no seed can be reached | Pod `mqtt-0` only; others never bootstrap |
| An odd number of nodes, 3 or 5, spread across zones | The session registry needs a raft quorum; 3 nodes tolerate 1 failure, 5 tolerate 2 | `replicas: 3`, topology spread across zones, PodDisruptionBudget `maxUnavailable: 1` |
| Never restart every node at once | The registry is kept in memory by default, so a node recovers it from the others; if all nodes restart together, session ownership is lost | OrderedReady rolling update gated on readiness; PDB |
| Graceful stop | SIGTERM drains the node (clients reconnect elsewhere and their sessions move) | `terminationGracePeriodSeconds` longer than the drain (the example waits up to 60s) |
| Health | `/healthz` returns 503 when the node has lost contact with a registry quorum or is draining | **Readiness** probe on `/healthz`, so the load balancer stops sending clients to that node. **Liveness** probe on TCP 1883 only: a node cut off from the registry should be taken out of the load balancer, not restarted |
| Load balancing | Any node accepts any client, and sessions follow the client | Layer 4 TCP load balancer, no session affinity |
| Metrics | `/metrics` serves Prometheus text | GKE: Managed Service for Prometheus `PodMonitoring`. VKE: kube-prometheus-stack or Grafana Cloud agent |

Cluster nodes should be in one region. Raft and claims (`ClaimTimeout`, `LeaseTimeout`) are
tuned for LAN latency. To serve several regions, run a separate cluster in each one, or bridge
them, rather than stretching one cluster across regions.

## In this repo

Stage 1 is built:

| Piece | Where | Notes |
|---|---|---|
| Node binary | [cmd/cluster](../cmd/cluster) | Every setting is a flag or an `MQTT_*` environment variable (`mochi-cluster -help` lists them). Clients are checked against an auth ledger file (`MQTT_AUTH_FILE`); allowing every client takes an explicit `MQTT_ALLOW_ALL=true`. Optional TLS listener (certificates reload when the files change), websocket listener, and gossip encryption (`MQTT_GOSSIP_KEY`). Admin http serves `/metrics`, `/stats`, `/healthz`, `/livez` and `/drain` |
| Image | [deploy/docker/Dockerfile.cluster](../deploy/docker/Dockerfile.cluster) | Distroless, non-root, about 21MB, cross-compiled for amd64 and arm64 |
| Helm chart | [deploy/helm/mochi-cluster](../deploy/helm/mochi-cluster) | StatefulSet, headless peer Service, client Service, PodDisruptionBudget, NetworkPolicy, auth and gossip key Secrets, PodMonitoring/ServiceMonitor. Refuses to render without auth configured |
| Smoke test | [deploy/kind/smoke.sh](../deploy/kind/smoke.sh) | Installs three nodes on kind, checks membership, sends QoS 1 messages between nodes with an authenticated client, checks a wrong password is refused, then does a rolling restart and checks again |
| CI | [.github/workflows/cluster.yml](../.github/workflows/cluster.yml) | Chart lint and schema validation, cluster unit tests, chaos suite (pushes, nightly, and pull requests touching the cluster or server), kind smoke test, and on pushes a multi-arch image pushed to `ghcr.io/<owner>/mochi-cluster`, tagged by commit, branch and version, with its digest in the run summary |

How nodes find each other: every node is given the headless Service's DNS name as its seed,
and resolves it itself, leaving out its own address. The library creates a new cluster when
it has no seeds, so this decides who may bootstrap. A node whose name ends in `-0` bootstraps
only if no other node resolves. Any other node waits up to `MQTT_JOIN_TIMEOUT` for one to
appear, then exits, and never starts a second cluster. The headless Service publishes nodes
which aren't ready yet (`publishNotReadyAddresses`), because a node can't become ready until
it has joined.

Probes: readiness is `/healthz`, so a node out of contact with the registry, or draining, is
taken out of the client Service. Liveness and startup are `/livez`, so such a node isn't
restarted. The startup probe allows for the join timeout.

Still to do:

1. **Terraform** in `deploy/terraform/{gcp,vultr}` for the Kubernetes cluster, registry, DNS
   and static IP, and per-cloud values files (`values-gcp.yaml`, `values-vultr.yaml`).
2. **Deploy jobs**: staging on pushes to the default branch, production on tags (see
   [CI/CD](#cicd)), deploying the image digest the `image` job publishes.
3. **Optional raft persistence** to a volume (raft-boltdb), so that restarting every node at
   once doesn't lose the session registry.
4. **Encrypt and authenticate peer links.** Forwarding and raft are plaintext, so the cluster
   relies on the private network and the NetworkPolicy. mTLS on both listeners (a
   `tls.Config` option in `cluster.Options`) would remove that dependency, and would allow
   node pools in different networks.

## GCP

| Piece | Choice |
|---|---|
| Kubernetes | GKE **Autopilot**, regional (nodes spread across three zones, no node pools to manage) |
| Images | Artifact Registry (or GHCR) |
| Load balancer | `Service` type `LoadBalancer`, which gives a regional passthrough Network Load Balancer. Reserve a static IP in Terraform and set `loadBalancerIP` |
| DNS | Cloud DNS record such as `mqtt.example.com` pointing to the static IP |
| TLS for clients | Terminate in the broker (cert-manager with Let's Encrypt, mounted as a secret), because the passthrough NLB doesn't terminate TLS |
| Metrics | Managed Service for Prometheus with `PodMonitoring` on port `admin` |
| CI credentials | **Workload Identity Federation**: GitHub's OIDC token is exchanged for a short-lived service account token, so no long-lived keys are stored in GitHub |
| Terraform state | GCS bucket |

If you prefer VMs:
a regional managed instance group on Container-Optimized OS, with seeds found from instance
metadata or a DNS name, and an internal health check on `/healthz`. That route takes more
custom work for rolling updates.

## Vultr

| Piece | Choice |
|---|---|
| Kubernetes | **VKE** (Vultr Kubernetes Engine), 3 nodes in one region, attached to a VPC |
| Images | Vultr Container Registry, or GHCR |
| Load balancer | `Service` type `LoadBalancer`; Vultr's cloud controller creates a Vultr Load Balancer. Set annotations for TCP protocol and the health check, for example `service.beta.kubernetes.io/vultr-loadbalancer-protocol: tcp` |
| DNS | Vultr DNS, or your existing DNS provider, through Terraform |
| TLS for clients | cert-manager in the broker, as on GCP |
| Metrics | kube-prometheus-stack in the cluster, or a Grafana Cloud agent |
| CI credentials | Vultr API key as a GitHub Environment secret (Vultr has no OIDC federation). Use a dedicated key, and rotate it. The kubeconfig is fetched with the key in each run, not stored |
| Terraform state | Vultr Object Storage (S3-compatible backend) |

Vultr regions don't have separate availability zones, so a VKE cluster sits in one location
(enable the HA control plane option). For higher availability, run a second cluster in
another region.

VM alternative: three instances in a VPC, created by Terraform, with cloud-init installing
Docker and a systemd unit running the image. Seeds are the other instances' private IPs from
Terraform. Deploys then need a rolling script over SSH that restarts one instance and waits
for `/healthz` before moving on. It works, but it's what Kubernetes already does for you.

## Other clouds

The same chart runs anywhere with managed Kubernetes and a TCP load balancer: AWS EKS (NLB),
Azure AKS, DigitalOcean DOKS, Linode LKE, Hetzner (k3s plus the hcloud cloud controller). Each
needs only its own Terraform module and a values file. Fly.io also supports UDP and a private
network between machines, but would need a different deploy path.

## CI/CD

```
PR ──► vet, lint, race tests ──► chaos suite (cluster/ changes) ──► build image (no push)
       helm lint + kubeconform ──► terraform plan (posted as a PR comment)

main ──► build + push image (tag: sha, digest recorded)
     ──► deploy to staging: GCP and Vultr in parallel (helm upgrade --atomic --wait)
     ──► smoke test: loadtest cross-node through each staging load balancer
     ──► terraform apply (staging)

tag v* ──► promote the same digest to production (GitHub Environment "production")
       ──► helm upgrade --atomic --wait, one cloud at a time
       ──► smoke test; --atomic rolls back on failure

nightly ──► loadtest benchmark against staging, compared with the last run
        ──► full chaos suite
```

- **Tests.** The existing `build.yml` already runs `go vet` and `go test -race ./...`. Add
  the chaos suite (`cd cluster/chaos && go test -tags chaos -timeout 30m`). GitHub's
  `ubuntu-latest` runners have Docker, so it runs as is. Run it on PRs that change `cluster/`,
  and nightly.
- **Build once.** Build the image only on `main`, push it to GHCR with
  `docker/build-push-action` (amd64 and arm64), and deploy by **digest**, so staging and
  production run identical bits. The current `docker` job only runs for `mochi-mqtt/server`
  and pushes to Docker Hub; change the condition and the registry for this fork.
- **Deploy.** `helm upgrade --install --atomic --wait --timeout 15m`. With OrderedReady and the
  readiness probe, Kubernetes restarts one pod at a time, waits for it to rejoin and report
  healthy, and only then moves on. `--atomic` rolls back if any pod doesn't become ready.
- **Smoke test.** Run [loadtest](../examples/benchmark/loadtest) publishing on one pod and
  subscribing on another (port-forward to two pods, or target the load balancer twice), and
  fail the deploy if delivery is below 100% at QoS 1. Also check `/stats` on each pod for the
  expected member count.
- **Approvals.** Production is a GitHub Environment. Leave it fully automatic, or add one
  required reviewer as a single click. That's the only manual step in a release.
- **Infrastructure.** Terraform `plan` on PRs that change `deploy/terraform`, `apply` on merge,
  with the production apply behind the same environment protection.
- **Option: GitOps.** If more clusters are added, install Argo CD or Flux in each cluster and
  have CI only push the image and commit the new digest to a `deploy/` values file. Each
  cluster then pulls its own changes, and CI holds no cluster credentials. For two clouds,
  pushing from Actions is simpler to start with.

## Manual steps

One time, per cloud:

1. Create the account or project and enable billing.
2. **GCP:** create the Workload Identity pool and provider for this GitHub repository, plus
   the deploy service account. This can be a small bootstrap Terraform run locally once.
   **Vultr:** create an API key and save it as a GitHub Environment secret.
3. Create the Terraform state bucket.
4. Point the DNS zone's nameservers at Cloud DNS or Vultr DNS, if they manage it.
5. Add the MQTT auth configuration and gossip key as secrets (or let Terraform create them in
   Secret Manager).

After that, infrastructure changes, deploys, rollbacks and benchmark runs all go through
pull requests and tags.

## Rollout order

1. ~~Repo work: `cmd/cluster`, image, chart, CI with a kind smoke test.~~ Done (stage 1).
   Next: Terraform modules and the deploy jobs.
2. GCP staging, end to end through CI.
3. Vultr staging with the same chart.
4. Production on one cloud, then the second, each with its own DNS name. Optionally, put
   weighted or failover DNS records across the two.
