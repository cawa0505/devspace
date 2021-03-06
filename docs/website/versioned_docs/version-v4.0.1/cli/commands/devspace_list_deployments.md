---
title: Command - devspace list deployments
sidebar_label: deployments
id: version-v4.0.1-devspace_list_deployments
original_id: devspace_list_deployments
---


Lists and shows the status of all deployments

## Synopsis


```
devspace list deployments [flags]
```

```
#######################################################
############# devspace list deployments ###############
#######################################################
Shows the status of all deployments
#######################################################
```
## Options

```
  -h, --help   help for deployments
```

### Options inherited from parent commands

```
      --debug                 Prints the stack trace if an error occurs
      --kube-context string   The kubernetes context to use
  -n, --namespace string      The kubernetes namespace to use
      --no-warn               If true does not show any warning when deploying into a different namespace or kube-context than before
  -p, --profile string        The devspace profile to use (if there is any)
      --silent                Run in silent mode and prevents any devspace log output except panics & fatals
  -s, --switch-context        Switches and uses the last kube context and namespace that was used to deploy the DevSpace project
      --var strings           Variables to override during execution (e.g. --var=MYVAR=MYVALUE)
```

## See Also

* [devspace list](../../cli/commands/devspace_list)	 - Lists configuration
