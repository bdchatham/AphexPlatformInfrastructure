import { Construct } from "constructs";
import { App, TerraformStack } from "cdktf";
import { KubernetesProvider } from "./.gen/providers/kubernetes/provider";
import { ConfigMapV1 } from "./.gen/providers/kubernetes/config-map-v1";

class TestStack extends TerraformStack {
  constructor(scope: Construct, id: string) {
    super(scope, id);

    // Configure Kubernetes provider to use in-cluster config
    new KubernetesProvider(this, "kubernetes", {
      configPath: undefined,
      inClusterConfig: true,
    });

    // Create a simple ConfigMap for testing
    new ConfigMapV1(this, "test-configmap", {
      metadata: {
        name: "pipeline-test-config",
        namespace: "default",
        labels: {
          "app": "pipeline-test",
          "managed-by": "cdktf",
          "test": "true",
        },
      },
      data: {
        "test-key": "test-value",
        "deployment-time": new Date().toISOString(),
        "message": "This ConfigMap was created by the pipeline test",
      },
    });
  }
}

const app = new App();
new TestStack(app, "test-pipeline-execution");
app.synth();
