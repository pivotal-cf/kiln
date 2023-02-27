package commands

import (
	"context"
	"fmt"
	vault "github.com/hashicorp/vault/api"
	vault_auth "github.com/hashicorp/vault/api/auth/ldap"
	"log"
)

func VaultGetCred(credential string) error {
	config := vault.DefaultConfig()

	config.Address = "https://runway-vault.eng.vmware.com/"
	authFromFile, err := vault_auth.NewLDAPAuth("nrohn", &vault_auth.Password{FromEnv: "VAULT_PASSWORD"})
	if err != nil {
		return fmt.Errorf("error initializing LDAPAuth with password file: %v", err)
	}

	client, err := vault.NewClient(config)
	loginRespFromFile, err := client.Auth().Login(context.TODO(), authFromFile)
	if err != nil {
		return fmt.Errorf("unable to initialize Vault client: %v", err)
	}
	if loginRespFromFile.Auth == nil || loginRespFromFile.Auth.ClientToken == "" {
		return fmt.Errorf("no authentication info returned by login")
	}

	client.SetToken(loginRespFromFile.Auth.ClientToken)
	ctx := context.Background()
	secret, err := client.KVv1("runway_concourse").Get(ctx, fmt.Sprintf("ppe-ci/%s", credential))
	if err != nil {
		return fmt.Errorf("unable to read secret: %v", err)
	}
	data, ok := secret.Data["dockerhub_username"]
	if !ok {
		return fmt.Errorf("no data found for key: %s", "dockerhub_username")
	}
	if val, ok := data.(string); ok {
		fmt.Println("secret data:", val)
	} else {
		return fmt.Errorf("value type assertion failed: %T %#v", secret.Data["password"], secret.Data["password"])
	}

	log.Println("Access granted!")
	return nil
}
