# Azure CNAB Driver

[![Build Status](https://dev.azure.com/deislabs/cnab-azure-driver/_apis/build/status/deislabs.cnab-azure-driver?branchName=main)](https://dev.azure.com/deislabs/cnab-azure-driver/_build/latest?definitionId=25&branchName=main)

The Azure CNAB Driver enables the *installation* of CNAB Bundle using [Azure Container Instance](https://azure.microsoft.com/en-gb/services/container-instances/) as an installation driver, this enables installation of a CNAB bundle from environments where using the Docker driver is impossible 

The main purpose of this driver is to enable CNAB operations to be performed using [Azure CloudShell](https://azure.microsoft.com/en-gb/features/cloud-shell/). 

## Requirements to use the Driver

You must have an [Azure account](https://azure.microsoft.com/free/) to use this driver.

Your Azure user account or the account that you use to execute the driver ([see below for details on authentication](#authentication-to-azure)) needs to have permission to create and use Azure resources for this driver to work. By default the driver will need to create and delete a resource group and create and delete a container group

By default it requires the ability to create\update\delete [Resource Groups](https://docs.microsoft.com/en-us/azure/role-based-access-control/resource-provider-operations#microsoftresources) and create\update\delete [Container Groups](https://docs.microsoft.com/en-us/azure/role-based-access-control/resource-provider-operations#microsoftcontainerinstance)

If you provide a resource group to be used by the driver either by setting a default resource group using `az configure -d group=<resource-group-name>` or by setting the environment variable `AZURE_CNAB_RESOURCE_GROUP`

## Getting Started

The easiest way to get started is to use [Azure Cloud Shell](https://shell.azure.com) with [Porter](). The driver should then work without any further configuration.

#### 1. Install the latest Porter and CNAB Azure Driver Releases.

```console
curl https://raw.githubusercontent.com/deislabs/cnab-azure-driver/main/install-in-azure-cloudshell.sh |/bin/bash
source .bashrc
```
This will install the latest porter and azure-cnab-driver releases and update `.bashrc` to include these in your path

#### 2. Install a sample bundle:

A simple test bundle that generates outputs can be run to validate the install:

```console
porter install test --tag deislabs/porter-example-exec-outputs-bundle:0.1.0 -d azure
porter show test

```

## Default Azure Location and Credentials

The environment variable `CNAB_AZURE_LOCATION` can be set to any region [where the ACI Service is available](https://azure.microsoft.com/en-us/global-infrastructure/services/?products=container-instances&regions=all), by default in CloudShell the driver will derive the location from the default location set in using `az configure -d location=<location>` otherwise it will use the users CloudShell location.

In CloudShell the credentials that you are logged in with will be used to create the ACI Container Group to run the invocation image and it will pick the current default subscription (this can be set or checked using `az account`), a specific subscription can be chosen by setting the environment Variable `CNAB_AZURE_SUBSCRIPTION_ID`to the subscription ID to be used. 

More details on alternative authentication approaches are specified [below](#authentication-to-azure). 

## Logging and tracing

To enable trace logs of the driver to be output to the console set the environment variable `CNAB_AZURE_VERBOSE` to `true`, to get details of the HTTP requests sent to Azure set the environment variable `AZURE_GO_SDK_LOG_LEVEL` to `INFO`, to include the request and response bodies of the requests set the value to `DEBUG`. Logs are also stored at `$HOME/.cnab-azure-driver/logs`

## Authentication to Azure

The ACI Driver can Authenticate to Azure using the following mechanisms and will evaluate them in this order:

1. Service Principal

Setting the environment variables `CNAB_AZURE_CLIENT_ID`, `CNAB_AZURE_CLIENT_SECRET` and `CNAB_AZURE_TENANT_ID` will cause the driver to attempt to login using those credentials as a service principal. More details on how to create a service prinicpal for authentication can be found [here](https://docs.microsoft.com/en-us/cli/azure/create-an-azure-service-principal-azure-cli?view=azure-cli-latest)

2. Device Code Flow

Setting the environment variables `CNAB_AZURE_APP_ID` and `CNAB_AZURE_TENANT_ID` will cause the driver to use the [Azure Device Code flow](https://docs.microsoft.com/en-us/azure/active-directory/develop/v2-oauth2-device-code), you need to set the AZURE_APP_ID variable to the applicationId of a native application that has been registered with Azure AAD and has access to read user profiles from Azure Graph. To register  an application see the documentation [here](https://docs.microsoft.com/en-us/azure/active-directory/develop/quickstart-register-app) and [here](https://docs.microsoft.com/en-us/azure/active-directory/develop/quickstart-configure-app-access-web-apis)

3. CloudShell 

If the driver is running in Azure CloudShell it will automatically login using the logged in users token if no environment variables are set.

4. MSI

If the driver is running in an environment where MSI is available (such as in a VM in Azure) , it will attempt to login using MSI, no configuration is necessary, the driver will detect the MSI endpoint and use it if it is available.

5. az cli

If the driver is running in an environment where az CLI is available then the driver will attempt to get an OAuth token using the CLI.

## ACI Container Group Identity

By default the ACI Container Group that is created to run the invocation image has no identity, in order to perform authenticated actions against resources credentials need to be presented to the invocation image. It is possible to have the ACI Container Group that executes the invocation image use [Managed Service Identity(MSI)](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/overview) . This enables the invocation image to be able to access the token for this identity and use it for bundle actions. The driver supports both System Assigned and User Assigned MSI. To use system assigned MSI set the environment variable `CNAB_AZURE_MSI_TYPE` to `system`. If no other environment variables are set the MSI will be assigned the Contributor role at the scope of the Resource Group that the ACI Container Group is created in, to override this behaviour the environment variable `CNAB_AZURE_SYSTEM_MSI_ROLE` can be set to the role required and `CNAB_AZURE_SYSTEM_MSI_SCOPE` can be set to set the scope for the assignment. Note that when using System MSI in order to prevent a race condition between  code in the bundle that relies on permissions being allocated to the MSI and the assignment of required permissions to the MSI the Container Group is first created using an alpine image. This allows for the system assigned MSI to be created and permissions assigned, once this is done the invocation image is launched. To use User Assigned MSI `CNAB_AZURE_MSI_TYPE` should be set to `user` and environment variable `CNAB_AZURE_USER_MSI_RESOURCE_ID` should be set to the Resource Id of the User Assigned MSI. You can also set the variable `CNAB_AZURE_PROPAGATE_CREDENTIALS` to propagate the Azure OAuth token from the local environment to the container in the environment variable `AZURE_ADAL_TOKEN`

## Resource Group and Location for the Container Group

The ACI Driver requires at least either `CNAB_AZURE_RESOURCE_GROUP` to be set to the name of an existing resource group or `CNAB_AZURE_LOCATION` to be set to the name of the region in which the ACI Container Group used to run the installation image will be created, however if running in CloudShell and neither of these values are set it will derive `CNAB_AZURE_LOCATION` from the default location set in the users az cli defaults (you can set this using `az configure -d location=<location>`) otherwise it will use the users CloudShell location.  The driver will create a new resource group in the region specified by the environment variable `CNAB_AZURE_LOCATION` if the environment variable `CNAB_AZURE_RESOURCE_GROUP` is not set or if the resource group specified in `CNAB_AZURE_RESOURCE_GROUP` does not exist.  If `CNAB_AZURE_RESOURCE_GROUP` is not set the name of the resource group will be auto-generated. If the resource group specified in `CNAB_AZURE_RESOURCE_GROUP` already exists then that resource group will be used. If the `CNAB_AZURE_RESOURCE_GROUP` exists and `CNAB_AZURE_LOCATION` is unset then the location of `CNAB_AZURE_RESOURCE_GROUP` will be used as for the location of the resources. The invocation image is also free to create resources in this resource group if it can acquire the correct permissions.

## ACI Container Group and Container Instance Naming

The container group and the container instance use the value of the environment variable `CNAB_AZURE_NAME` for their names, if this is not set then a name is generated automatically, prefixed with `cnab-`, it is not possible to use different names for the group and the container, they will always use the same name.

## Deleting Resources

By default the driver will delete the container group that it creates and also the resource group if it creates it (pre-existing resource groups are not deleted), this behaviour can be changed by setting the environment variable `CNAB_AZURE_DO_NOT_DELETE` to true. This can be useful for debugging or if you know that the invocation image is going to create resources in the same resource group. The container group property `restartPolicy` is set to `Never`.

## Debugging the Invocation Image

In order to debug issues with the execution of the invocation set the environment variable `CNAB_AZURE_DEBUG_CONTAINER` to true, this will cause the command `tail -f \dev\null`to be run in the container, you can then connect to the instance by executing `az container exec -g <resource-group-name> -n <container-group-instance> --exec-command /bin/sh`. You can find the resource group and container name in the log file.

## Dealing with Bundle Outputs

Some bundles create outputs, the driver captures these in an Azure File Share, the details of the file share to be user should be provided in the environment variables  `CNAB_AZURE_STATE_FILESHARE,CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME ,CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY`, in CloudShell the users clouddrive is used for these data.

## Environment Variables

|  Environment Variable 	| Description  	|
|---	|---	|
| CNAB_AZURE_VERBOSE  	| Verbose output - set to true to enable  	|
| CNAB_AZURE_CLIENT_ID  	| AAD Client ID for Azure account authentication - used to authenticate to Azure using Service Principal for ACI creation  	|
| CNAB_AZURE_CLIENT_SECRET  	|  AAD Client Secret for Azure account authentication - used to authenticate to Azure using Service Principal for ACI creation 	|
| CNAB_AZURE_TENANT_ID  	|  Azure AAD Tenant Id Azure account authentication - used to authenticate to Azure using Service Principal or Device Code for ACI creation 	|
| CNAB_AZURE_APP_ID  	|  Azure Application Id - this is the application to be used when authenticating to Azure using device flow	|
| CNAB_AZURE_SUBSCRIPTION_ID    | Azure Subscription Id - this is the subscription to be used for ACI creation, if not specified the first (random) subscription is used    | 
| CNAB_AZURE_RESOURCE_GROUP  	|   The name of the existing Resource Group to create the ACI instance in, if not specified a Resource Group will be created, if specified and it does not exist a new resource group with this name will be created	|
| CNAB_AZURE_LOCATION  	|   The location in which to create the ACI Container Group and Resource Group	|
| CNAB_AZURE_NAME  	|   The name of the ACI instance to create - if not specified a name will be generated	|
| CNAB_AZURE_DELETE_RESOURCES  	|  Set to false so as not to delete the RG and ACI container group created, default is true - useful for debugging - only deletes RG if it was created by the driver 	|
| CNAB_AZURE_MSI_TYPE  	|   This can be set to either `user` or `system` This value is presented to the invocation image container as `AZURE_MSI_TYPE`|
| CNAB_AZURE_SYSTEM_MSI_ROLE  	|  If `CNAB_AZURE_SYSTEM_MSI_ROLE` is set to `system` this defines the role to be assigned to System MSI User, if this is null or empty then the role defaults to `Contributor`	|
| CNAB_AZURE_SYSTEM_MSI_SCOPE  	|  If `CNAB_AZURE_SYSTEM_MSI_ROLE` is set to `system` this defines the scope to apply the role to System MSI User - if this is null or empty then the scope will be Resource Group that the ACI Instance is being created |
| CNAB_AZURE_USER_MSI_RESOURCE_ID  	|  If `CNAB_AZURE_SYSTEM_MSI_ROLE` is set to `user` this is required and should contain the resource_id of the User MSI to be used This value is presented to the invocation image container as `AZURE_USER_MSI_RESOURCE_ID`</li>|
| CNAB_AZURE_PROPAGATE_CREDENTIALS | Default false. If this is set to true and MSI is not being used then any credentials set\used to create the ACI instance are also propagated to the invocation image in an ENV variable as follows : <br/><ul><li> `CNAB_AZURE_CLIENT_ID` becomes `AZURE_CLIENT_ID`</li><li>`CNAB_AZURE_CLIENT_SECRET` becomes `AZURE_CLIENT_SECRET`</li></ul><br/>Tenant and subscription details used are presented to the invocation image container as follows <br/><ul><li>`CNAB_AZURE_TENANT_ID` becomes `AZURE_TENANT_ID`</li><li>`CNAB_AZURE_SUBSCRIPTION_ID` becomes `AZURE_SUBSCRIPTION_ID`</li></ul><br/> In addition if the driver uses CloudShell or az cli for authentication then the ADAL Token used by those tools will be propagated as a json object in the environment variable `AZURE_ADAL_TOKEN`. If the CNAB package being invoked defines environment variables with matching names then any values provided will overwrite the values from the ACI Driver. |
| CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH 	|   If this is set to true then `CNAB_AZURE_CLIENT_ID` and `CNAB_AZURE_CLIENT_SECRET`	are used for authentication with the registry containing the invocation image, `CNAB_AZURE_REGISTRY_USERNAME` and `CNAB_AZURE_REGISTRY_PASSWORD` should not be set|
| CNAB_AZURE_REGISTRY_USERNAME 	|  Username to authenticate to Registry for invocation image	|
| CNAB_AZURE_REGISTRY_PASSWORD  	|  Password to authenticate to Registry for invocation image 	|
| CNAB_AZURE_STATE_FILESHARE     |  The File Share for Azure State volume |
| CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME | The Storage Account for the Azure State File Share |
| CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY |  The Storage Key for the Azure State File Share |
| CNAB_AZURE_STATE_PATH | The local path relative to the mount point where state can be stored - this is combined with the state mount point and set as environment variable `STATE_PATH` on the ACI instance and can be used by a bundle to persist filesystem data |
| CNAB_AZURE_DELETE_OUTPUTS_FROM_FILESHARE | Bundle outputs are written to an Azure file share, setting this variable to false will cause the driver not to clean these up after the action is finished. |
| CNAB_AZURE_DEBUG_CONTAINER | Setting this to true enables connection to the container instance to debug issues, it causes the command /cnab/app/run with tail -f /dev/null to be run in the invocation image. |