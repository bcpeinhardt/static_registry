# Static Terraform Registry

This is an implementation of the terraform module registry protocol which fetches resources on
build rather than at runtime. 

### How to run

To build the binary, run `make build`. This will produce a binary called `registry`, which
you can simply run. All resources required for the registry to operate are embedded in the binary 
itself.
With the binary running, open another terminal window and cd into the `terraform_test` directory.
You will see a `main.tf` which simply loads the code-server modules from the registry on localhost.
Go ahead and delete any other files/folders in the terraform_test directory.
Run `terraform init && terraform apply` and you should see the module source get downloaded correctly
and the terraform.tfstate file get created.

### How does this work?

On build, the modules repo is cloned from github and embedded in the go binary.
We do some sneaky renaming to get the .git folder past go:embed.
When the binary is run, the embedded FS is copied into an in memory file system,
which is read by go-git. We then drive the registry's responses by performing
git operations on the repo (but in memory, no shelling out required).