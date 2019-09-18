package main

import (
	"encoding/json"
	"fmt"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/alidns"
	certmanager_v1alpha1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/issuer/acme/dns/util"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"strings"

	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	// This will register our custom DNS provider with the webhook serving
	// library, making it available as an API under the provided GroupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(GroupName,
		&aliyunDNSProviderSolver{},
	)
}

// aliyunDNSProviderSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/jetstack/cert-manager/pkg/acme/webhook.Solver`
// interface.
type aliyunDNSProviderSolver struct {
	client     *kubernetes.Clientset
	dnsClients map[string]*alidns.Client
}

// aliyunDNSProviderConfig is a structure that is used to decode into when
// solving a DNS01 challenge.
// This information is provided by cert-manager, and may be a reference to
// additional configuration that's needed to solve the challenge for this
// particular certificate or issuer.
// This typically includes references to Secret resources containing DNS
// provider credentials, in cases where a 'multi-tenant' DNS solver is being
// created.
// If you do *not* require per-issuer or per-certificate configuration to be
// provided to your webhook, you can skip decoding altogether in favour of
// using CLI flags or similar to provide configuration.
// You should not include sensitive information here. If credentials need to
// be used by your provider here, you should reference a Kubernetes Secret
// resource and fetch these credentials using a Kubernetes clientset.
type aliyunDNSProviderConfig struct {
	RegionId           string                                 `json:"regionId"`
	AccessKeyId        string                                 `json:"accessKeyId"`
	AccessKeySecret    string                                 `json:"accessKeySecret"`
	AccessKeySecretRef certmanager_v1alpha1.SecretKeySelector `json:"accessKeySecretRef"`
	TTL                *int                                   `json:"ttl"`
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *aliyunDNSProviderSolver) Name() string {
	return "alidns"
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *aliyunDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	fmt.Printf("Decoded configuration %v", cfg)

	client, err := c.getDnsClient(ch, cfg)
	if err != nil {
		return err
	}
	fmt.Printf("fqdn:[%s] zone:[%s]\n", ch.ResolvedFQDN, ch.ResolvedZone)
	domainName := c.extractDomainName(ch.ResolvedZone)
	request := alidns.CreateAddDomainRecordRequest()
	request.DomainName = domainName
	request.RR = c.extractRecordName(ch.ResolvedFQDN, domainName)
	request.Type = "TXT"
	request.Value = ch.Key
	request.TTL = requests.NewInteger(*cfg.TTL)
	response, err := client.AddDomainRecord(request)

	if err != nil {
		return err
	}

	fmt.Printf("Response: %v", response)
	return nil
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *aliyunDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	fmt.Printf("Decoded configuration %v", cfg)

	client, err := c.getDnsClient(ch, cfg)
	if err != nil {
		return err
	}
	records, err := c.findTxtRecords(client, ch.ResolvedZone, ch.ResolvedFQDN)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.Value != ch.Key {
			continue
		}
		request := alidns.CreateDeleteDomainRecordRequest()
		request.RecordId = record.RecordId
		_, err = client.DeleteDomainRecord(request)
		if err != nil {
			return fmt.Errorf("alidns: %v", err)
		}
	}
	return nil
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *aliyunDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	///// UNCOMMENT THE BELOW CODE TO MAKE A KUBERNETES CLIENTSET AVAILABLE TO
	///// YOUR CUSTOM DNS PROVIDER

	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	c.client = cl

	///// END OF CODE TO MAKE KUBERNETES CLIENTSET AVAILABLE

	c.dnsClients = make(map[string]*alidns.Client)

	return nil
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extapi.JSON) (aliyunDNSProviderConfig, error) {
	cfg := aliyunDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}

func (c *aliyunDNSProviderSolver) getDnsClient(ch *v1alpha1.ChallengeRequest, cfg aliyunDNSProviderConfig) (*alidns.Client, error) {
	accessKeyId := cfg.AccessKeyId
	client, ok := c.dnsClients[accessKeyId]

	if ok {
		return client, nil
	}

	accessKeySecret := cfg.AccessKeySecret
	if accessKeySecret == "" {
		ref := cfg.AccessKeySecretRef
		if ref.Key == "" {
			return nil, fmt.Errorf("no accessKeySecret for %q in secret '%s/%s'", ref.Name, ref.Key, ch.ResourceNamespace)
		}
		if ref.Name == "" {
			return nil, fmt.Errorf("no accessKeySecret for %q in secret '%s/%s'", ref.Name, ref.Key, ch.ResourceNamespace)
		}
		secret, err := c.client.CoreV1().Secrets(ch.ResourceNamespace).Get(ref.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		accessKeySecretRef, ok := secret.Data[ref.Key]
		if !ok {
			return nil, fmt.Errorf("no accessKeySecret for %q in secret '%s/%s'", ref.Name, ref.Key, ch.ResourceNamespace)
		}
		accessKeySecret = fmt.Sprintf("%s", accessKeySecretRef)
	}
	client, err := alidns.NewClientWithAccessKey(
		cfg.RegionId, // 您的可用区ID
		accessKeyId,  // 您的Access Key ID
		accessKeySecret,
	)

	if err != nil {
		return nil, err
	}
	c.dnsClients[cfg.AccessKeyId] = client

	client.OpenLogger()
	return client, nil
}

func (c *aliyunDNSProviderSolver) findTxtRecords(client *alidns.Client, zone, fqdn string) ([]alidns.Record, error) {
	_, zoneName, err := c.getHostedZone(client, zone)
	if err != nil {
		return nil, err
	}

	request := alidns.CreateDescribeDomainRecordsRequest()
	request.DomainName = zoneName
	request.PageSize = requests.NewInteger(500)

	var records []alidns.Record

	result, err := client.DescribeDomainRecords(request)
	if err != nil {
		return records, fmt.Errorf("API call has failed: %v", err)
	}

	recordName := c.extractRecordName(fqdn, zoneName)
	for _, record := range result.DomainRecords.Record {
		if record.RR == recordName {
			records = append(records, record)
		}
	}
	return records, nil
}

func (c *aliyunDNSProviderSolver) getHostedZone(client *alidns.Client, zone string) (string, string, error) {
	request := alidns.CreateDescribeDomainsRequest()

	var domains []alidns.Domain
	startPage := 1

	for {
		request.PageNumber = requests.NewInteger(startPage)

		response, err := client.DescribeDomains(request)
		if err != nil {
			return "", "", fmt.Errorf("API call failed: %v", err)
		}

		domains = append(domains, response.Domains.Domain...)

		if response.PageNumber*response.PageSize >= response.TotalCount {
			break
		}

		startPage++
	}

	authZone, err := util.FindZoneByFqdn(zone, util.RecursiveNameservers)
	if err != nil {
		return "", "", err
	}

	var hostedZone alidns.Domain
	for _, zone := range domains {
		if zone.DomainName == util.UnFqdn(authZone) {
			hostedZone = zone
		}
	}

	if hostedZone.DomainId == "" {
		return "", "", fmt.Errorf("zone %s not found in AliDNS for domain %s", authZone, zone)
	}
	return fmt.Sprintf("%v", hostedZone.DomainId), hostedZone.DomainName, nil
}

func (c *aliyunDNSProviderSolver) extractRecordName(fqdn, domain string) string {
	if idx := strings.Index(fqdn, "."+domain); idx != -1 {
		return fqdn[:idx]
	}
	return util.UnFqdn(fqdn)
}

func (c *aliyunDNSProviderSolver) extractDomainName(zone string) string {
	authZone, err := util.FindZoneByFqdn(zone, util.RecursiveNameservers)
	if err != nil {
		return zone
	}
	return util.UnFqdn(authZone)
}
