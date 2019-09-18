package azure

import (
	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
	"github.com/Azure/azure-sdk-for-go/services/containerinstance/mgmt/2018-10-01/containerinstance"
	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2015-11-01/subscriptions"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2017-05-10/resources"
	"github.com/Azure/go-autorest/autorest"
)

// GetSubscriptionsClient gets a Subscriptions Management Client
func GetSubscriptionsClient(authorizer autorest.Authorizer, userAgent string) subscriptions.Client {
	subscriptionClient := subscriptions.NewClient()
	subscriptionClient.Authorizer = authorizer
	subscriptionClient.AddToUserAgent(userAgent)
	return subscriptionClient
}

// GetRoleDefinitionsClient gets a RoleDefinitions Management Client
func GetRoleDefinitionsClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) authorization.RoleDefinitionsClient {
	roleDefinitionsClient := authorization.NewRoleDefinitionsClient(subscriptionID)
	roleDefinitionsClient.Authorizer = authorizer
	roleDefinitionsClient.AddToUserAgent(userAgent)
	return roleDefinitionsClient
}

// GetRoleAssignmentClient gets a RoleAssignment Management Client
func GetRoleAssignmentClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) authorization.RoleAssignmentsClient {
	roleAssignmentsClient := authorization.NewRoleAssignmentsClient(subscriptionID)
	roleAssignmentsClient.Authorizer = authorizer
	roleAssignmentsClient.AddToUserAgent(userAgent)
	return roleAssignmentsClient
}

// GetUserAssignedIdentitiesClient gets a UserAssignedIdentities Management Client
func GetUserAssignedIdentitiesClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) msi.UserAssignedIdentitiesClient {
	userAssignedIdentitiesClient := msi.NewUserAssignedIdentitiesClient(subscriptionID)
	userAssignedIdentitiesClient.Authorizer = authorizer
	userAssignedIdentitiesClient.AddToUserAgent(userAgent)
	return userAssignedIdentitiesClient
}

// GetContainerGroupsClient gets a ContainerGroups Management Client
func GetContainerGroupsClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) containerinstance.ContainerGroupsClient {
	containerGroupsClient := containerinstance.NewContainerGroupsClient(subscriptionID)
	containerGroupsClient.Authorizer = authorizer
	containerGroupsClient.AddToUserAgent(userAgent)
	return containerGroupsClient
}

// GetContainerClient gets a Container Management Client
func GetContainerClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) containerinstance.ContainerClient {
	containerClient := containerinstance.NewContainerClient(subscriptionID)
	containerClient.Authorizer = authorizer
	containerClient.AddToUserAgent(userAgent)
	return containerClient
}

// GetGroupsClient gets a Resource Group Management Client
func GetGroupsClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) resources.GroupsClient {
	groupsClient := resources.NewGroupsClient(subscriptionID)
	groupsClient.Authorizer = authorizer
	groupsClient.AddToUserAgent(userAgent)
	return groupsClient
}

// GetProvidersClient gets a Providers Management Client
func GetProvidersClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) resources.ProvidersClient {
	providersClient := resources.NewProvidersClient(subscriptionID)
	providersClient.Authorizer = authorizer
	providersClient.AddToUserAgent(userAgent)
	return providersClient
}
