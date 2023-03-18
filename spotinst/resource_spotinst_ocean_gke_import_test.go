package spotinst

import (
	"context"
	"fmt"
	log "github.com/sourcegraph-ce/logrus"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/spotinst/spotinst-sdk-go/service/ocean/providers/gcp"
	"github.com/spotinst/spotinst-sdk-go/spotinst"
	"github.com/terraform-providers/terraform-provider-spotinst/spotinst/commons"
)

var GcpClusterName = "terraform-tests-do-not-delete"

func init() {
	resource.AddTestSweepers("resource_spotinst_ocean_gke_import", &resource.Sweeper{
		Name: "resource_spotinst_ocean_gke_import",
		F:    testSweepOceanGKEImportCluster,
	})
}

func testSweepOceanGKEImportCluster(region string) error {
	client, err := getProviderClient("gcp")
	if err != nil {
		return fmt.Errorf("error getting client: %v", err)
	}

	conn := client.(*Client).ocean.CloudProviderGCP()

	input := &gcp.ListClustersInput{}
	if resp, err := conn.ListClusters(context.Background(), input); err != nil {
		return fmt.Errorf("error getting list of clusters to sweep")
	} else {
		if len(resp.Clusters) == 0 {
			log.Printf("[INFO] No clusters to sweep")
		}
		for _, cluster := range resp.Clusters {
			if strings.Contains(spotinst.StringValue(cluster.Name), "terraform-acc-tests-") {
				if _, err := conn.DeleteCluster(context.Background(), &gcp.DeleteClusterInput{ClusterID: cluster.ID}); err != nil {
					return fmt.Errorf("unable to delete group %v in sweep", spotinst.StringValue(cluster.ID))
				} else {
					log.Printf("Sweeper deleted %v\n", spotinst.StringValue(cluster.ID))
				}
			}
		}
	}
	return nil
}

func createOceanGKEImportResourceName(name string) string {
	return fmt.Sprintf("%v.%v", string(commons.OceanGKEImportResourceName), name)
}

func testOceanGKEImportDestroy(s *terraform.State) error {
	client := testAccProviderGCP.Meta().(*Client)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != string(commons.OceanGKEImportResourceName) {
			continue
		}
		input := &gcp.ReadClusterInput{ClusterID: spotinst.String(rs.Primary.ID)}
		resp, err := client.ocean.CloudProviderGCP().ReadCluster(context.Background(), input)
		if err == nil && resp != nil && resp.Cluster != nil {
			return fmt.Errorf("cluster still exists")
		}
	}
	return nil
}

func testCheckOceanGKEImportAttributes(cluster *gcp.Cluster, expectedName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if spotinst.StringValue(cluster.Name) != expectedName {
			return fmt.Errorf("bad content: %v", cluster.Name)
		}
		return nil
	}
}

func testCheckOceanGKEImportExists(cluster *gcp.Cluster, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no resource ID is set")
		}
		client := testAccProviderGCP.Meta().(*Client)
		input := &gcp.ReadClusterInput{ClusterID: spotinst.String(rs.Primary.ID)}
		resp, err := client.ocean.CloudProviderGCP().ReadCluster(context.Background(), input)
		if err != nil {
			return err
		}
		//
		// During import, Spotinst API sets the 'name' field to be as the GCP cluster name
		// GCP cluster name cannot be changed after creation while resource have dynamic names per test
		//
		//if spotinst.StringValue(resp.Cluster.Name) != rs.Primary.Attributes["name"] {
		//	return fmt.Errorf("Cluster not found: %+v,\n %+v\n", resp.Cluster, rs.Primary.Attributes)
		//}
		*cluster = *resp.Cluster
		return nil
	}
}

type OceanGKEImportMetadata struct {
	clusterName          string
	provider             string
	fieldsToAppend       string
	updateBaselineFields bool
}

func createOceanGKEImportTerraform(clusterMeta *OceanGKEImportMetadata) string {
	if clusterMeta == nil {
		return ""
	}

	if clusterMeta.provider == "" {
		clusterMeta.provider = "gcp"
	}

	template :=
		`provider "gcp" {
	token   = "fake"
	account = "fake"
	}
	`
	if clusterMeta.updateBaselineFields {
		format := testBaselineOceanGKEImportConfig_Update
		template += fmt.Sprintf(format,
			clusterMeta.clusterName,
			clusterMeta.provider,
			clusterMeta.fieldsToAppend,
		)
	} else {
		format := testBaselineOceanGKEImportConfig_Create
		template += fmt.Sprintf(format,
			clusterMeta.clusterName,
			clusterMeta.provider,
			clusterMeta.fieldsToAppend,
		)

	}

	log.Printf("Terraform [%v] template:\n%v", clusterMeta.clusterName, template)
	return template
}

// region Ocean GKE Import: Baseline
func TestAccSpotinstOceanGKEImport_Baseline(t *testing.T) {
	spotClusterName := "terraform-tests-do-not-delete"
	resourceName := createOceanGKEImportResourceName(spotClusterName)

	var cluster gcp.Cluster
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t, "gcp") },
		Providers:    TestAccProviders,
		CheckDestroy: testOceanGKEImportDestroy,

		Steps: []resource.TestStep{
			{
				Config: createOceanGKEImportTerraform(&OceanGKEImportMetadata{
					clusterName: spotClusterName,
				}),
				Check: resource.ComposeTestCheckFunc(
					testCheckOceanGKEImportExists(&cluster, resourceName),
					testCheckOceanGKEImportAttributes(&cluster, GcpClusterName),
					resource.TestCheckResourceAttr(resourceName, "whitelist.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "whitelist.0", "n1-standard-1"),
					resource.TestCheckResourceAttr(resourceName, "whitelist.1", "n1-standard-2"),
				),
			},
			{
				Config: createOceanGKEImportTerraform(&OceanGKEImportMetadata{
					clusterName:          spotClusterName,
					updateBaselineFields: true}),
				Check: resource.ComposeTestCheckFunc(
					testCheckOceanGKEImportExists(&cluster, resourceName),
					testCheckOceanGKEImportAttributes(&cluster, GcpClusterName),
					resource.TestCheckResourceAttr(resourceName, "whitelist.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "whitelist.0", "n1-standard-1"),
				),
			},
		},
	})
}

const testBaselineOceanGKEImportConfig_Create = `
resource "` + string(commons.OceanGKEImportResourceName) + `" "%v" {
 provider = "%v"

 cluster_name = "terraform-tests-do-not-delete"
 location     = "us-central1-a"

 whitelist = ["n1-standard-1", "n1-standard-2"]
 %v
}

`

const testBaselineOceanGKEImportConfig_Update = `
resource "` + string(commons.OceanGKEImportResourceName) + `" "%v" {
 provider = "%v"

 cluster_name = "terraform-tests-do-not-delete"
 location     = "us-central1-a"

 whitelist = ["n1-standard-1"]
 %v
}

`

//endregion

//region Ocean GKE Import: BackendServices
func TestAccSpotinstOceanGKEImport_BackendServices(t *testing.T) {
	spotClusterName := "terraform-tests-do-not-delete"
	resourceName := createOceanGKEImportResourceName(spotClusterName)

	var cluster gcp.Cluster
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t, "gcp") },
		Providers:    TestAccProviders,
		CheckDestroy: testOceanGKEImportDestroy,

		Steps: []resource.TestStep{
			{
				Config: createOceanGKEImportTerraform(&OceanGKEImportMetadata{
					clusterName:    spotClusterName,
					fieldsToAppend: testBackendServicesOceanGKEImportConfig_Create,
				}),
				Check: resource.ComposeTestCheckFunc(
					testCheckOceanGKEImportExists(&cluster, resourceName),
					testCheckOceanGKEImportAttributes(&cluster, GcpClusterName),
					resource.TestCheckResourceAttr(resourceName, "backend_services.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.233520595.location_type", "global"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.233520595.service_name", "terraform-bs-dont-delete"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.233520595.named_ports.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.233520595.named_ports.571950593.name", "http"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.233520595.named_ports.571950593.ports.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.233520595.named_ports.571950593.ports.0", "80"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.233520595.named_ports.571950593.ports.1", "8080"),
				),
			},
			{
				Config: createOceanGKEImportTerraform(&OceanGKEImportMetadata{
					clusterName:    spotClusterName,
					fieldsToAppend: testBackendServicesOceanGKEImportConfig_Update,
				}),
				Check: resource.ComposeTestCheckFunc(
					testCheckOceanGKEImportExists(&cluster, resourceName),
					testCheckOceanGKEImportAttributes(&cluster, GcpClusterName),
					resource.TestCheckResourceAttr(resourceName, "backend_services.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.3017970807.location_type", "global"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.3017970807.service_name", "terraform-bs-dont-delete"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.3017970807.named_ports.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.3017970807.named_ports.2171153412.name", "https"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.3017970807.named_ports.2171153412.ports.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "backend_services.3017970807.named_ports.2171153412.ports.0", "443"),
				),
			},
		},
	})
}

const testBackendServicesOceanGKEImportConfig_Create = `
 backend_services {
     service_name = "terraform-bs-dont-delete"
     location_type = "global"

     named_ports  {
       name = "http"
       ports = [
         80,
         8080]
     }
   }


`

const testBackendServicesOceanGKEImportConfig_Update = `
 backend_services  {
     service_name = "terraform-bs-dont-delete"
     location_type = "global"

     named_ports {
       name = "https"
       ports = [443]
     }
   }
`

// endregion

// region Ocean GKE Import: Scheduling
func TestAccSpotinstOceanGKEImport_Scheduling(t *testing.T) {
	spotClusterName := "terraform-tests-do-not-delete"
	resourceName := createOceanGKEImportResourceName(spotClusterName)

	var cluster gcp.Cluster
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t, "gcp") },
		Providers:    TestAccProviders,
		CheckDestroy: testOceanGKEImportDestroy,

		Steps: []resource.TestStep{
			{
				ResourceName: resourceName,
				Config: createOceanGKEImportTerraform(&OceanGKEImportMetadata{
					clusterName:    spotClusterName,
					fieldsToAppend: testOceanGKEScheduling_Create,
				}),
				Check: resource.ComposeTestCheckFunc(
					testCheckOceanGKEImportExists(&cluster, resourceName),
					testCheckOceanGKEImportAttributes(&cluster, GcpClusterName),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.1137025357.shutdown_hours.0.is_enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.1137025357.shutdown_hours.0.time_windows.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.1137025357.shutdown_hours.0.time_windows.0", "Fri:15:30-Sat:17:30"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.1137025357.tasks.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.1137025357.tasks.0.cron_expression", "0 1 1 * *"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.1137025357.tasks.0.is_enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.1137025357.tasks.0.task_type", "clusterRoll"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.1137025357.tasks.0.batch_size_percentage", "50"),
				),
			},
			{
				ResourceName: resourceName,
				Config: createOceanGKEImportTerraform(&OceanGKEImportMetadata{
					clusterName:    spotClusterName,
					fieldsToAppend: testOceanGKEScheduling_Update,
				}),
				Check: resource.ComposeTestCheckFunc(
					testCheckOceanGKEImportExists(&cluster, resourceName),
					testCheckOceanGKEImportAttributes(&cluster, GcpClusterName),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.2134430294.shutdown_hours.0.is_enabled", "false"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.2134430294.shutdown_hours.0.time_windows.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.2134430294.shutdown_hours.0.time_windows.0", "Fri:15:30-Sat:18:30"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.2134430294.tasks.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.2134430294.tasks.0.cron_expression", "0 1 * * *"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.2134430294.tasks.0.is_enabled", "false"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.2134430294.tasks.0.task_type", "clusterRoll"),
					resource.TestCheckResourceAttr(resourceName, "scheduled_task.2134430294.tasks.0.batch_size_percentage", "20"),
				),
			},
		},
	})
}

const testOceanGKEScheduling_Create = `
  scheduled_task  {
     shutdown_hours  {
       is_enabled = true
       time_windows = ["Fri:15:30-Sat:17:30"]
     }
     tasks  {
       is_enabled = true
       cron_expression = "0 1 1 * *"
       task_type = "clusterRoll"
       batch_size_percentage = 50
     }
   }


`

const testOceanGKEScheduling_Update = `
  scheduled_task  {
     shutdown_hours  {
       is_enabled = false
       time_windows = ["Fri:15:30-Sat:18:30"]
     }
     tasks  {
       is_enabled = false
       cron_expression = "0 1 * * *"
       task_type = "clusterRoll"
       batch_size_percentage = 20
     }
   }

`

// endregion

// region Ocean GKE Import: autoscaler
func TestAccSpotinstOceanGKEImport_Autoscaler(t *testing.T) {
	spotClusterName := "terraform-tests-do-not-delete"
	resourceName := createOceanGKEImportResourceName(spotClusterName)

	var cluster gcp.Cluster
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t, "gcp") },
		Providers:    TestAccProviders,
		CheckDestroy: testOceanGKEImportDestroy,

		Steps: []resource.TestStep{
			{
				ResourceName: resourceName,
				Config: createOceanGKEImportTerraform(&OceanGKEImportMetadata{
					clusterName:    spotClusterName,
					fieldsToAppend: testOceanGKEAutoscaler_Create,
				}),
				Check: resource.ComposeTestCheckFunc(
					testCheckOceanGKEImportExists(&cluster, resourceName),
					testCheckOceanGKEImportAttributes(&cluster, GcpClusterName),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.auto_headroom_percentage", "15"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.cooldown", "360"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.down.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.down.0.evaluation_periods", "340"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.down.0.max_scale_down_percentage", "12"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.headroom.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.headroom.0.cpu_per_unit", "512"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.headroom.0.gpu_per_unit", "2"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.headroom.0.memory_per_unit", "256"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.headroom.0.num_of_units", "1"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.is_auto_config", "true"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.is_enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.resource_limits.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.resource_limits.0.max_memory_gib", "10"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.resource_limits.0.max_vcpu", "512"),
				),
			},
			{
				ResourceName: resourceName,
				Config: createOceanGKEImportTerraform(&OceanGKEImportMetadata{
					clusterName:    spotClusterName,
					fieldsToAppend: testOceanGKEAutoscaler_Update,
				}),
				Check: resource.ComposeTestCheckFunc(
					testCheckOceanGKEImportExists(&cluster, resourceName),
					testCheckOceanGKEImportAttributes(&cluster, GcpClusterName),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.auto_headroom_percentage", "20"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.cooldown", "300"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.down.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.down.0.evaluation_periods", "200"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.down.0.max_scale_down_percentage", "6"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.headroom.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.headroom.0.cpu_per_unit", "256"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.headroom.0.gpu_per_unit", "3"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.headroom.0.memory_per_unit", "512"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.headroom.0.num_of_units", "2"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.is_auto_config", "false"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.is_enabled", "false"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.resource_limits.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.resource_limits.0.max_memory_gib", "5"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.resource_limits.0.max_vcpu", "256"),
				),
			},
			{
				ResourceName: resourceName,
				Config: createOceanGKEImportTerraform(&OceanGKEImportMetadata{
					clusterName:    spotClusterName,
					fieldsToAppend: testScalerGKEConfig_EmptyFields,
				}),
				Check: resource.ComposeTestCheckFunc(
					testCheckOceanGKEImportExists(&cluster, resourceName),
					testCheckOceanGKEImportAttributes(&cluster, GcpClusterName),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.down.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.headroom.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "autoscaler.0.resource_limits.#", "0"),
				),
			},
		},
	})
}

const testOceanGKEAutoscaler_Create = `

    autoscaler {
    is_enabled     = true
    is_auto_config = true
    cooldown       = 360
    auto_headroom_percentage = 15

    headroom {
      cpu_per_unit    = 512
      gpu_per_unit    = 2
      memory_per_unit = 256
      num_of_units    = 1
    }

    down {
      evaluation_periods = 340
      max_scale_down_percentage = 12
    }

    resource_limits {
      max_vcpu       = 512
      max_memory_gib = 10
    }
  }

`

const testOceanGKEAutoscaler_Update = `

    autoscaler {
    is_enabled     = false
    is_auto_config = false
    cooldown       = 300
    auto_headroom_percentage = 20

    headroom {
      cpu_per_unit    = 256
      gpu_per_unit    = 3
      memory_per_unit = 512
      num_of_units    = 2
    }

    down {
      evaluation_periods = 200
      max_scale_down_percentage = 6
    }

    resource_limits {
      max_vcpu       = 256
      max_memory_gib = 5
    }
  }
`

const testScalerGKEConfig_EmptyFields = `
// --- AUTOSCALER -----------------
autoscaler {}
// --------------------------------
`

// endregion
