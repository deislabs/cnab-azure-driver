package azure

import (
	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
	"github.com/Azure/azure-sdk-for-go/services/containerinstance/mgmt/2018-10-01/containerinstance"
	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2015-11-01/subscriptions"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2017-05-10/resources"
	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	"github.com/Azure/go-autorest/autorest"
)

// GetSubscriptionsClient gets a Subscriptions Management Client
func GetSubscriptionsClient(authorizer autorest.Authorizer, userAgent string) (*subscriptions.Client, error) {
	subscriptionClient := subscriptions.NewClient()
	if err := setupClient(&subscriptionClient.BaseClient.Client, userAgent, authorizer); err != nil {
		return nil, err
	}

	return &subscriptionClient, nil
}

// GetRoleDefinitionsClient gets a RoleDefinitions Management Client
func GetRoleDefinitionsClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) (*authorization.RoleDefinitionsClient, error) {
	roleDefinitionsClient := authorization.NewRoleDefinitionsClient(subscriptionID)
	if err := setupClient(&roleDefinitionsClient.BaseClient.Client, userAgent, authorizer); err != nil {
		return nil, err
	}

	return &roleDefinitionsClient, nil
}

// GetRoleAssignmentClient gets a RoleAssignment Management Client
func GetRoleAssignmentClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) (*authorization.RoleAssignmentsClient, error) {
	roleAssignmentsClient := authorization.NewRoleAssignmentsClient(subscriptionID)
	if err := setupClient(&roleAssignmentsClient.BaseClient.Client, userAgent, authorizer); err != nil {
		return nil, err
	}

	return &roleAssignmentsClient, nil
}

// GetUserAssignedIdentitiesClient gets a UserAssignedIdentities Management Client
func GetUserAssignedIdentitiesClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) (*msi.UserAssignedIdentitiesClient, error) {
	userAssignedIdentitiesClient := msi.NewUserAssignedIdentitiesClient(subscriptionID)
	if err := setupClient(&userAssignedIdentitiesClient.BaseClient.Client, userAgent, authorizer); err != nil {
		return nil, err
	}

	return &userAssignedIdentitiesClient, nil
}

// GetContainerGroupsClient gets a ContainerGroups Management Client
func GetContainerGroupsClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) (*containerinstance.ContainerGroupsClient, error) {
	containerGroupsClient := containerinstance.NewContainerGroupsClient(subscriptionID)
	if err := setupClient(&containerGroupsClient.BaseClient.Client, userAgent, authorizer); err != nil {
		return nil, err
	}

	return &containerGroupsClient, nil
}

// GetContainerClient gets a Container Management Client
func GetContainerClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) (*containerinstance.ContainerClient, error) {
	containerClient := containerinstance.NewContainerClient(subscriptionID)
	if err := setupClient(&containerClient.BaseClient.Client, userAgent, authorizer); err != nil {
		return nil, err
	}

	return &containerClient, nil
}

// GetGroupsClient gets a Resource Group Management Client
func GetGroupsClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) (*resources.GroupsClient, error) {
	groupsClient := resources.NewGroupsClient(subscriptionID)
	if err := setupClient(&groupsClient.BaseClient.Client, userAgent, authorizer); err != nil {
		return nil, err
	}

	return &groupsClient, nil
}

// GetProvidersClient gets a Providers Management Client
func GetProvidersClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) (*resources.ProvidersClient, error) {
	providersClient := resources.NewProvidersClient(subscriptionID)
	if err := setupClient(&providersClient.BaseClient.Client, userAgent, authorizer); err != nil {
		return nil, err
	}

	return &providersClient, nil
}

// GetStorageAccountsClient gets a Providers Management Client
func GetStorageAccountsClient(subscriptionID string, authorizer autorest.Authorizer, userAgent string) (*storage.AccountsClient, error) {
	accountsClient := storage.NewAccountsClient(subscriptionID)
	if err := setupClient(&accountsClient.BaseClient.Client, userAgent, authorizer); err != nil {
		return nil, err
	}

	return &accountsClient, nil
}

func setupClient(client *autorest.Client, userAgent string, authorizer autorest.Authorizer) error {
	client.Authorizer = authorizer
	if err := client.AddToUserAgent(userAgent); err != nil {
		return err
	}
	return nil
}
