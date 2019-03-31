# gce-terminator

gce-terminator is an agent that periodically retrieves instances for an
unmanaged instance group and deletes any instances that are not in a pending or
running state.

## Usage

```
$ gce-terminator --interval 30s --instance-group firecracker-ci --gcp-zone us-east1-b --gcp-project endocrimes-buildkite
```
