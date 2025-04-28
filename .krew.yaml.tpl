apiVersion: krew.googlecontainertools.github.com/v1alpha2
kind: Plugin
metadata:
  name: resource-backup
spec:
  version: ${TAG}
  homepage: https://github.com/zak905/kubectl-resource-backup
  shortDescription: Save Kubernetes resources to disk
  description: |
    Backs up Kubernetes objects (including CRDs) to the local file system. 
    Before saving any resource, some additional processing is done to remove:
    - the status stanza if the object has any.
    - the server generated fields from the object metadata. 
    - any field with a null value.
    The aims is to make the saved objects look like the original creation request.
  caveats: |
    The fields that has a default value are not removed (unlike the neat plugin)  
    because it's not possible to make a distinction between a value set 
    by a creation/update request and a value set by a controller 
    or a mutating admission webhook.
  platforms:
  - selector:
      matchLabels:
        os: darwin
        arch: amd64
    uri: https://github.com/zak905/kubectl-resource-backup/releases/download/${TAG}/kubectl-resource-backup_darwin_amd64.tar.gz
    sha256: ${DARWIN_AMD64_SHA256}
    bin: kubectl-resource-backup
  - selector:
      matchLabels:
        os: darwin
        arch: arm64
    uri: https://github.com/zak905/kubectl-resource-backup/releases/download/${TAG}/kubectl-resource-backup_darwin_arm64.tar.gz
    sha: ${DARWIN_ARM64_SHA256}
    bin: kubectl-resource-backup
  - selector:
      matchLabels:
        os: linux
        arch: amd64
    uri: https://github.com/zak905/kubectl-resource-backup/releases/download/${TAG}/kubectl-resource-backup_linux_amd64.tar.gz
    sha: ${LINUX_AMD64_SHA256}
    bin: kubectl-resource-backup
  - selector:
      matchLabels:
        os: linux
        arch: arm64
    uri: https://github.com/zak905/kubectl-resource-backup/releases/download/${TAG}/kubectl-resource-backup_linux_arm64.tar.gz
    sha: ${LINUX_ARM64_SHA256}
    bin: kubectl-resource-backup
  - selector:
      matchLabels:
        os: windows
        arch: amd64
    uri: https://github.com/zak905/kubectl-resource-backup/releases/download/${TAG}/kubectl-resource-backup_windows_amd64.tar.gz
    sha: ${WINDOWS_AMD64_SHA256}
    bin: kubectl-resource-backup.exe
  - selector:
      matchLabels:
        os: windows
        arch: arm64
    uri: https://github.com/zak905/kubectl-resource-backup/releases/download/${TAG}/kubectl-resource-backup_windows_arm64.tar.gz
    sha: ${WINDOWS_ARM64_SHA256}
    bin: kubectl-resource-backup.exe