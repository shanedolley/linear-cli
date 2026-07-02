package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/shanedolley/lincli/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Releases (the other Tier 4a domain) are intentionally absent: writes require
// Linear's Business plan (FEATURE_NOT_ACCESSIBLE 'releaseManagement'), so
// shipping release commands would only produce commands that always fail. This
// file covers the available Customers/CRM domain.

var customerCmd = &cobra.Command{
	Use:   "customer",
	Short: "Manage customers (CRM)",
	Long: `Manage customers and their needs, plus the customer status and tier catalogs.

Customer records can contain personal data; --json prints it unredacted.

Examples:
  lincli customer list
  lincli customer get "Acme Inc"
  lincli customer create --name "Acme Inc" --tier Enterprise
  lincli customer merge <source> <target>
  lincli customer need list --customer "Acme Inc"
  lincli customer status list
  lincli customer tier list`,
}

// --- customer ---

var customerListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List customers",
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		filter := &api.CustomerFilter{}
		newerThan, _ := cmd.Flags().GetString("newer-than")
		createdAt, err := utils.ParseTimeExpression(newerThan)
		if err != nil {
			return fmt.Errorf("Invalid newer-than value: %v", err)
		}
		if createdAt != "" {
			filter.CreatedAt = &api.DateComparator{Gte: &createdAt}
		}

		resp, err := api.ListCustomers(ctx, client, filter, limitPtr, nil, nil)
		if err != nil {
			return fmt.Errorf("Failed to list customers: %v", err)
		}

		if resp.Customers == nil || len(resp.Customers.Nodes) == 0 {
			output.Info("No customers found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.Customers.Nodes)
			return nil
		}

		headers := []string{"ID", "Name", "Status", "Tier", "Needs"}
		rows := make([][]string, len(resp.Customers.Nodes))
		for i, node := range resp.Customers.Nodes {
			f := node.CustomerFields
			tier := "-"
			if f.Tier != nil {
				tier = f.Tier.Name
			}
			status := "-"
			if f.Status != nil {
				status = f.Status.Name
			}
			rows[i] = []string{
				f.Id,
				truncateString(f.Name, 30),
				status,
				tier,
				fmt.Sprintf("%.0f", f.ApproximateNeedCount),
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d customers\n", color.New(color.FgGreen).Sprint("✓"), len(resp.Customers.Nodes))
		}
		return nil
	},
}

var customerGetCmd = &cobra.Command{
	Use:     "get <customer>",
	Aliases: []string{"show"},
	Short:   "Get a customer by name or ID",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		id, err := resolveCustomer(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		resp, err := api.GetCustomer(ctx, client, id)
		if err != nil {
			return fmt.Errorf("Failed to get customer: %v", err)
		}
		f := resp.Customer.CustomerFields

		if jsonOut {
			output.JSON(resp.Customer)
			return nil
		}

		if plaintext {
			fmt.Printf("# %s\n", f.Name)
			fmt.Printf("- **ID**: %s\n", f.Id)
			fmt.Printf("- **Needs**: %.0f\n", f.ApproximateNeedCount)
			return nil
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Customer:"), f.Name)
		fmt.Printf("  ID:     %s\n", f.Id)
		if f.Status != nil {
			fmt.Printf("  Status: %s\n", f.Status.Name)
		}
		if f.Tier != nil {
			fmt.Printf("  Tier:   %s\n", f.Tier.Name)
		}
		if f.Owner != nil {
			fmt.Printf("  Owner:  %s\n", f.Owner.Name)
		}
		fmt.Printf("  Needs:  %.0f\n", f.ApproximateNeedCount)
		if len(f.Domains) > 0 {
			fmt.Printf("  Domains: %v\n", f.Domains)
		}
		return nil
	},
}

var customerCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a customer",
	Long:    `Create a customer. --name is required.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return errors.New("Name is required (--name)")
		}

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		input := &api.CustomerCreateInput{Name: name}

		if cmd.Flags().Changed("owner") {
			owner, _ := cmd.Flags().GetString("owner")
			ownerID, err := resolveUser(ctx, client, cache, owner)
			if err != nil {
				return err
			}
			input.OwnerId = &ownerID
		}
		if cmd.Flags().Changed("status") {
			status, _ := cmd.Flags().GetString("status")
			statusID, err := resolveCustomerStatus(ctx, client, cache, status)
			if err != nil {
				return err
			}
			input.StatusId = &statusID
		}
		if cmd.Flags().Changed("tier") {
			tier, _ := cmd.Flags().GetString("tier")
			tierID, err := resolveCustomerTier(ctx, client, cache, tier)
			if err != nil {
				return err
			}
			input.TierId = &tierID
		}
		if cmd.Flags().Changed("domains") {
			domains, _ := cmd.Flags().GetStringSlice("domains")
			input.Domains = domains
		}
		if cmd.Flags().Changed("revenue") {
			revenue, _ := cmd.Flags().GetInt("revenue")
			input.Revenue = &revenue
		}

		resp, err := api.CustomerCreate(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to create customer: %v", err)
		}
		if resp.CustomerCreate == nil || !resp.CustomerCreate.Success {
			return errors.New("Failed to create customer")
		}

		if jsonOut {
			output.JSON(resp.CustomerCreate.Customer)
		} else {
			output.Success(fmt.Sprintf("Created customer %s", resp.CustomerCreate.Customer.CustomerFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var customerUpdateCmd = &cobra.Command{
	Use:   "update <customer>",
	Short: "Update a customer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		id, err := resolveCustomer(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		input := &api.CustomerUpdateInput{}
		changed := false

		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
			changed = true
		}
		if cmd.Flags().Changed("owner") {
			owner, _ := cmd.Flags().GetString("owner")
			ownerID, err := resolveUser(ctx, client, cache, owner)
			if err != nil {
				return err
			}
			input.OwnerId = &ownerID
			changed = true
		}
		if cmd.Flags().Changed("status") {
			status, _ := cmd.Flags().GetString("status")
			statusID, err := resolveCustomerStatus(ctx, client, cache, status)
			if err != nil {
				return err
			}
			input.StatusId = &statusID
			changed = true
		}
		if cmd.Flags().Changed("tier") {
			tier, _ := cmd.Flags().GetString("tier")
			tierID, err := resolveCustomerTier(ctx, client, cache, tier)
			if err != nil {
				return err
			}
			input.TierId = &tierID
			changed = true
		}
		if cmd.Flags().Changed("revenue") {
			revenue, _ := cmd.Flags().GetInt("revenue")
			input.Revenue = &revenue
			changed = true
		}

		if !changed {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.CustomerUpdate(ctx, client, id, input)
		if err != nil {
			return fmt.Errorf("Failed to update customer: %v", err)
		}
		if resp.CustomerUpdate == nil || !resp.CustomerUpdate.Success {
			return errors.New("Failed to update customer")
		}

		if jsonOut {
			output.JSON(resp.CustomerUpdate.Customer)
		} else {
			output.Success(fmt.Sprintf("Updated customer %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var customerDeleteCmd = &cobra.Command{
	Use:     "delete <customer>",
	Aliases: []string{"rm"},
	Short:   "Delete a customer",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		id, err := resolveCustomer(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		resp, err := api.CustomerDelete(ctx, client, id)
		if err != nil {
			return fmt.Errorf("Failed to delete customer: %v", err)
		}
		if resp.CustomerDelete == nil || !resp.CustomerDelete.Success {
			return errors.New("Failed to delete customer")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": id})
		} else {
			output.Success(fmt.Sprintf("Deleted customer %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var customerMergeCmd = &cobra.Command{
	Use:   "merge <source> <target>",
	Short: "Merge one customer into another",
	Long: `Merge the source customer into the target. The source's needs move to the
target and the source is archived. Executes immediately with no prompt.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		sourceID, err := resolveCustomer(ctx, client, cache, args[0])
		if err != nil {
			return err
		}
		targetID, err := resolveCustomer(ctx, client, cache, args[1])
		if err != nil {
			return err
		}

		resp, err := api.CustomerMerge(ctx, client, sourceID, targetID)
		if err != nil {
			return fmt.Errorf("Failed to merge customers: %v", err)
		}
		if resp.CustomerMerge == nil || !resp.CustomerMerge.Success {
			return errors.New("Failed to merge customers")
		}

		survivor := resp.CustomerMerge.Customer.CustomerFields
		if jsonOut {
			output.JSON(resp.CustomerMerge.Customer)
		} else {
			output.Success(fmt.Sprintf("Merged %s into %s (survivor: %s)", args[0], args[1], survivor.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var customerUpsertCmd = &cobra.Command{
	Use:   "upsert",
	Short: "Create or update a customer keyed on an external ID",
	Long: `Create or update a customer, keyed on --external-id. If a customer with that
external identifier exists it is updated, otherwise a new one is created.
--external-id and --name are required.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		externalID, _ := cmd.Flags().GetString("external-id")
		if externalID == "" {
			return errors.New("External ID is required (--external-id)")
		}
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return errors.New("Name is required (--name)")
		}

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		input := &api.CustomerUpsertInput{
			ExternalId: &externalID,
			Name:       &name,
		}
		if cmd.Flags().Changed("tier-name") {
			tierName, _ := cmd.Flags().GetString("tier-name")
			input.TierName = &tierName
		}
		if cmd.Flags().Changed("domains") {
			domains, _ := cmd.Flags().GetStringSlice("domains")
			input.Domains = domains
		}

		resp, err := api.CustomerUpsert(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to upsert customer: %v", err)
		}
		if resp.CustomerUpsert == nil || !resp.CustomerUpsert.Success {
			return errors.New("Failed to upsert customer")
		}

		if jsonOut {
			output.JSON(resp.CustomerUpsert.Customer)
		} else {
			output.Success(fmt.Sprintf("Upserted customer %s", resp.CustomerUpsert.Customer.CustomerFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var customerUnsyncCmd = &cobra.Command{
	Use:   "unsync <customer>",
	Short: "Disconnect a customer from its external source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		id, err := resolveCustomer(ctx, client, cache, args[0])
		if err != nil {
			return err
		}

		resp, err := api.CustomerUnsync(ctx, client, id)
		if err != nil {
			return fmt.Errorf("Failed to unsync customer: %v", err)
		}
		if resp.CustomerUnsync == nil || !resp.CustomerUnsync.Success {
			return errors.New("Failed to unsync customer")
		}

		if jsonOut {
			output.JSON(resp.CustomerUnsync.Customer)
		} else {
			output.Success(fmt.Sprintf("Unsynced customer %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// --- customer need ---

var customerNeedCmd = &cobra.Command{
	Use:   "need",
	Short: "Manage customer needs",
	Long:  `Manage customer needs: pieces of customer feedback tied to issues or projects.`,
}

var customerNeedListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List customer needs",
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		filter := &api.CustomerNeedFilter{}
		if customer, _ := cmd.Flags().GetString("customer"); customer != "" {
			customerID, err := resolveCustomer(ctx, client, cache, customer)
			if err != nil {
				return err
			}
			filter.Customer = &api.NullableCustomerFilter{Id: &api.IDComparator{Eq: &customerID}}
		}

		resp, err := api.ListCustomerNeeds(ctx, client, filter, limitPtr, nil, nil)
		if err != nil {
			return fmt.Errorf("Failed to list customer needs: %v", err)
		}

		if resp.CustomerNeeds == nil || len(resp.CustomerNeeds.Nodes) == 0 {
			output.Info("No customer needs found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.CustomerNeeds.Nodes)
			return nil
		}

		headers := []string{"ID", "Customer", "Summary", "Issue", "Priority"}
		rows := make([][]string, len(resp.CustomerNeeds.Nodes))
		for i, node := range resp.CustomerNeeds.Nodes {
			f := node.CustomerNeedFields
			customer := "-"
			if f.Customer != nil {
				customer = f.Customer.Name
			}
			issue := "-"
			if f.Issue != nil {
				issue = f.Issue.Identifier
			}
			rows[i] = []string{
				f.Id,
				truncateString(customer, 20),
				truncateString(customerNeedSummary(f), 35),
				issue,
				fmt.Sprintf("%.0f", f.Priority),
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d needs\n", color.New(color.FgGreen).Sprint("✓"), len(resp.CustomerNeeds.Nodes))
		}
		return nil
	},
}

var customerNeedGetCmd = &cobra.Command{
	Use:     "get <need-id>",
	Aliases: []string{"show"},
	Short:   "Get a customer need",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		resp, err := api.GetCustomerNeed(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to get customer need: %v", err)
		}
		f := resp.CustomerNeed.CustomerNeedFields

		if jsonOut {
			output.JSON(resp.CustomerNeed)
			return nil
		}

		fmt.Printf("%s %s\n", color.New(color.FgCyan, color.Bold).Sprint("Need:"), f.Id)
		if f.Customer != nil {
			fmt.Printf("  Customer: %s\n", f.Customer.Name)
		}
		if f.Issue != nil {
			fmt.Printf("  Issue:    %s\n", f.Issue.Identifier)
		}
		fmt.Printf("  Priority: %.0f\n", f.Priority)
		if summary := customerNeedSummary(f); summary != "-" {
			fmt.Printf("\n%s\n", summary)
		}
		return nil
	},
}

var customerNeedCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a customer need",
	Long: `Create a customer need tied to a customer and linked to an issue or project.
--customer, --body, and one of --issue/--project are required.

The need is created with a Markdown body. Sourcing a need from an existing
attachment, comment, or attachment URL (the API's attachmentId, commentId, and
attachmentUrl inputs) is not yet wired; pass the content with --body instead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		customer, _ := cmd.Flags().GetString("customer")
		if customer == "" {
			return errors.New("Customer is required (--customer)")
		}
		body, _ := cmd.Flags().GetString("body")
		if body == "" {
			return errors.New("Body is required (--body)")
		}

		issue, _ := cmd.Flags().GetString("issue")
		project, _ := cmd.Flags().GetString("project")
		if err := validateCustomerNeedTarget(issue, project); err != nil {
			return err
		}

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		customerID, err := resolveCustomer(ctx, client, cache, customer)
		if err != nil {
			return err
		}

		input := &api.CustomerNeedCreateInput{
			CustomerId: &customerID,
			Body:       &body,
		}
		if cmd.Flags().Changed("priority") {
			priority, _ := cmd.Flags().GetFloat64("priority")
			input.Priority = &priority
		}
		if cmd.Flags().Changed("issue") {
			issue, _ := cmd.Flags().GetString("issue")
			issueID := resolveParentIssueID(ctx, client, issue, plaintext, jsonOut)
			input.IssueId = &issueID
		}
		if cmd.Flags().Changed("project") {
			project, _ := cmd.Flags().GetString("project")
			projectID, err := resolveProject(ctx, client, cache, project)
			if err != nil {
				return err
			}
			input.ProjectId = &projectID
		}

		resp, err := api.CustomerNeedCreate(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to create customer need: %v", err)
		}
		if resp.CustomerNeedCreate == nil || !resp.CustomerNeedCreate.Success {
			return errors.New("Failed to create customer need")
		}

		if jsonOut {
			output.JSON(resp.CustomerNeedCreate.Need)
		} else {
			output.Success(fmt.Sprintf("Created customer need %s", resp.CustomerNeedCreate.Need.CustomerNeedFields.Id), plaintext, jsonOut)
		}
		return nil
	},
}

var customerNeedUpdateCmd = &cobra.Command{
	Use:   "update <need-id>",
	Short: "Update a customer need",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		input := &api.CustomerNeedUpdateInput{}
		changed := false

		if cmd.Flags().Changed("body") {
			body, _ := cmd.Flags().GetString("body")
			input.Body = &body
			changed = true
		}
		if cmd.Flags().Changed("priority") {
			priority, _ := cmd.Flags().GetFloat64("priority")
			input.Priority = &priority
			changed = true
		}
		if cmd.Flags().Changed("issue") {
			issue, _ := cmd.Flags().GetString("issue")
			issueID := resolveParentIssueID(ctx, client, issue, plaintext, jsonOut)
			input.IssueId = &issueID
			changed = true
		}
		if cmd.Flags().Changed("project") {
			project, _ := cmd.Flags().GetString("project")
			projectID, err := resolveProject(ctx, client, cache, project)
			if err != nil {
				return err
			}
			input.ProjectId = &projectID
			changed = true
		}

		if !changed {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.CustomerNeedUpdate(ctx, client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to update customer need: %v", err)
		}
		if resp.CustomerNeedUpdate == nil || !resp.CustomerNeedUpdate.Success {
			return errors.New("Failed to update customer need")
		}

		if jsonOut {
			output.JSON(resp.CustomerNeedUpdate.Need)
		} else {
			output.Success(fmt.Sprintf("Updated customer need %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var customerNeedDeleteCmd = &cobra.Command{
	Use:     "delete <need-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a customer need",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		resp, err := api.CustomerNeedDelete(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to delete customer need: %v", err)
		}
		if resp.CustomerNeedDelete == nil || !resp.CustomerNeedDelete.Success {
			return errors.New("Failed to delete customer need")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted customer need %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var customerNeedArchiveCmd = &cobra.Command{
	Use:   "archive <need-id>",
	Short: "Archive a customer need",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		resp, err := api.CustomerNeedArchive(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to archive customer need: %v", err)
		}
		if resp.CustomerNeedArchive == nil || !resp.CustomerNeedArchive.Success {
			return errors.New("Failed to archive customer need")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Archived customer need %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var customerNeedUnarchiveCmd = &cobra.Command{
	Use:     "unarchive <need-id>",
	Aliases: []string{"restore"},
	Short:   "Restore an archived customer need",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		resp, err := api.CustomerNeedUnarchive(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to unarchive customer need: %v", err)
		}
		if resp.CustomerNeedUnarchive == nil || !resp.CustomerNeedUnarchive.Success {
			return errors.New("Failed to unarchive customer need")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Unarchived customer need %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// --- customer status ---

var customerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Manage the customer status catalog",
}

var customerStatusListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List customer statuses",
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		limit := 250
		resp, err := api.ListCustomerStatuses(ctx, client, &limit, nil, nil)
		if err != nil {
			return fmt.Errorf("Failed to list customer statuses: %v", err)
		}

		if resp.CustomerStatuses == nil || len(resp.CustomerStatuses.Nodes) == 0 {
			output.Info("No customer statuses found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.CustomerStatuses.Nodes)
			return nil
		}

		headers := []string{"ID", "Name", "Color"}
		rows := make([][]string, len(resp.CustomerStatuses.Nodes))
		for i, node := range resp.CustomerStatuses.Nodes {
			f := node.CustomerStatusFields
			rows[i] = []string{f.Id, f.Name, f.Color}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)
		return nil
	},
}

var customerStatusCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a customer status",
	Long:    `Create a customer status. --name and --color are required.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		color, _ := cmd.Flags().GetString("color")
		if name == "" || color == "" {
			return errors.New("Name and color are required (--name, --color)")
		}

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		input := &api.CustomerStatusCreateInput{Name: &name, Color: color}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
		}

		resp, err := api.CustomerStatusCreate(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to create customer status: %v", err)
		}
		if resp.CustomerStatusCreate == nil || !resp.CustomerStatusCreate.Success {
			return errors.New("Failed to create customer status")
		}

		if jsonOut {
			output.JSON(resp.CustomerStatusCreate.Status)
		} else {
			output.Success(fmt.Sprintf("Created customer status %s", resp.CustomerStatusCreate.Status.CustomerStatusFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var customerStatusUpdateCmd = &cobra.Command{
	Use:   "update <status-id>",
	Short: "Update a customer status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		input := &api.CustomerStatusUpdateInput{}
		changed := false
		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
			changed = true
		}
		if cmd.Flags().Changed("color") {
			color, _ := cmd.Flags().GetString("color")
			input.Color = &color
			changed = true
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
			changed = true
		}

		if !changed {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.CustomerStatusUpdate(ctx, client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to update customer status: %v", err)
		}
		if resp.CustomerStatusUpdate == nil || !resp.CustomerStatusUpdate.Success {
			return errors.New("Failed to update customer status")
		}

		if jsonOut {
			output.JSON(resp.CustomerStatusUpdate.Status)
		} else {
			output.Success(fmt.Sprintf("Updated customer status %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var customerStatusDeleteCmd = &cobra.Command{
	Use:     "delete <status-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a customer status",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		resp, err := api.CustomerStatusDelete(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to delete customer status: %v", err)
		}
		if resp.CustomerStatusDelete == nil || !resp.CustomerStatusDelete.Success {
			return errors.New("Failed to delete customer status")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted customer status %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// --- customer tier ---

var customerTierCmd = &cobra.Command{
	Use:   "tier",
	Short: "Manage the customer tier catalog",
}

var customerTierListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List customer tiers",
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		limit := 250
		resp, err := api.ListCustomerTiers(ctx, client, &limit, nil, nil)
		if err != nil {
			return fmt.Errorf("Failed to list customer tiers: %v", err)
		}

		if resp.CustomerTiers == nil || len(resp.CustomerTiers.Nodes) == 0 {
			output.Info("No customer tiers found", plaintext, jsonOut)
			return nil
		}

		if jsonOut {
			output.JSON(resp.CustomerTiers.Nodes)
			return nil
		}

		headers := []string{"ID", "Name", "Color"}
		rows := make([][]string, len(resp.CustomerTiers.Nodes))
		for i, node := range resp.CustomerTiers.Nodes {
			f := node.CustomerTierFields
			rows[i] = []string{f.Id, f.Name, f.Color}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)
		return nil
	},
}

var customerTierCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a customer tier",
	Long:    `Create a customer tier. --name and --color are required.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		color, _ := cmd.Flags().GetString("color")
		if name == "" || color == "" {
			return errors.New("Name and color are required (--name, --color)")
		}

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		input := &api.CustomerTierCreateInput{Name: &name, Color: color}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
		}

		resp, err := api.CustomerTierCreate(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to create customer tier: %v", err)
		}
		if resp.CustomerTierCreate == nil || !resp.CustomerTierCreate.Success {
			return errors.New("Failed to create customer tier")
		}

		if jsonOut {
			output.JSON(resp.CustomerTierCreate.Tier)
		} else {
			output.Success(fmt.Sprintf("Created customer tier %s", resp.CustomerTierCreate.Tier.CustomerTierFields.Name), plaintext, jsonOut)
		}
		return nil
	},
}

var customerTierUpdateCmd = &cobra.Command{
	Use:   "update <tier-id>",
	Short: "Update a customer tier",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		input := &api.CustomerTierUpdateInput{}
		changed := false
		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			input.Name = &name
			changed = true
		}
		if cmd.Flags().Changed("color") {
			color, _ := cmd.Flags().GetString("color")
			input.Color = &color
			changed = true
		}
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			input.Description = &description
			changed = true
		}

		if !changed {
			return errors.New("No updates specified. Use flags to specify what to update.")
		}

		resp, err := api.CustomerTierUpdate(ctx, client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to update customer tier: %v", err)
		}
		if resp.CustomerTierUpdate == nil || !resp.CustomerTierUpdate.Success {
			return errors.New("Failed to update customer tier")
		}

		if jsonOut {
			output.JSON(resp.CustomerTierUpdate.Tier)
		} else {
			output.Success(fmt.Sprintf("Updated customer tier %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var customerTierDeleteCmd = &cobra.Command{
	Use:     "delete <tier-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a customer tier",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := customerClient()
		if err != nil {
			return err
		}

		resp, err := api.CustomerTierDelete(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to delete customer tier: %v", err)
		}
		if resp.CustomerTierDelete == nil || !resp.CustomerTierDelete.Success {
			return errors.New("Failed to delete customer tier")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Deleted customer tier %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// customerClient builds the authenticated client and context shared by the
// customer subcommands.
func customerClient() (graphql.Client, context.Context, error) {
	client, err := newGraphQLClient()
	if err != nil {
		return nil, nil, err
	}
	return client, context.Background(), nil
}

// validateCustomerNeedTarget enforces Linear's rule that a customer need must
// link to an issue or a project: customerNeedCreate rejects a need with neither
// (Argument Validation Error). At least one of the two references is required.
func validateCustomerNeedTarget(issue, project string) error {
	if issue == "" && project == "" {
		return fmt.Errorf("a linked issue or project is required: pass --issue or --project")
	}
	return nil
}

// customerNeedSummary renders a short human-readable summary of a need: its
// manual body when present, otherwise the content extracted from the source
// attachment, otherwise "-".
func customerNeedSummary(f api.CustomerNeedFields) string {
	if f.Body != nil && *f.Body != "" {
		return *f.Body
	}
	if f.Content != nil && *f.Content != "" {
		return *f.Content
	}
	return "-"
}

func init() {
	rootCmd.AddCommand(customerCmd)

	customerCmd.AddCommand(customerListCmd)
	customerCmd.AddCommand(customerGetCmd)
	customerCmd.AddCommand(customerCreateCmd)
	customerCmd.AddCommand(customerUpdateCmd)
	customerCmd.AddCommand(customerDeleteCmd)
	customerCmd.AddCommand(customerMergeCmd)
	customerCmd.AddCommand(customerUpsertCmd)
	customerCmd.AddCommand(customerUnsyncCmd)
	customerCmd.AddCommand(customerNeedCmd)
	customerCmd.AddCommand(customerStatusCmd)
	customerCmd.AddCommand(customerTierCmd)

	customerNeedCmd.AddCommand(customerNeedListCmd)
	customerNeedCmd.AddCommand(customerNeedGetCmd)
	customerNeedCmd.AddCommand(customerNeedCreateCmd)
	customerNeedCmd.AddCommand(customerNeedUpdateCmd)
	customerNeedCmd.AddCommand(customerNeedDeleteCmd)
	customerNeedCmd.AddCommand(customerNeedArchiveCmd)
	customerNeedCmd.AddCommand(customerNeedUnarchiveCmd)

	customerStatusCmd.AddCommand(customerStatusListCmd)
	customerStatusCmd.AddCommand(customerStatusCreateCmd)
	customerStatusCmd.AddCommand(customerStatusUpdateCmd)
	customerStatusCmd.AddCommand(customerStatusDeleteCmd)

	customerTierCmd.AddCommand(customerTierListCmd)
	customerTierCmd.AddCommand(customerTierCreateCmd)
	customerTierCmd.AddCommand(customerTierUpdateCmd)
	customerTierCmd.AddCommand(customerTierDeleteCmd)

	// customer
	customerListCmd.Flags().IntP("limit", "l", 50, "Maximum number of customers to fetch")
	customerListCmd.Flags().StringP("newer-than", "n", "all_time", "Show customers created after this time (e.g. 3_months_ago; default all_time)")

	customerCreateCmd.Flags().String("name", "", "Customer name (required)")
	customerCreateCmd.Flags().String("owner", "", "Owner (email, name, 'me', or ID)")
	customerCreateCmd.Flags().String("status", "", "Status name or ID")
	customerCreateCmd.Flags().String("tier", "", "Tier name or ID")
	customerCreateCmd.Flags().StringSlice("domains", nil, "Email domains (repeatable or comma-separated)")
	customerCreateCmd.Flags().Int("revenue", 0, "Annual revenue")

	customerUpdateCmd.Flags().String("name", "", "New name")
	customerUpdateCmd.Flags().String("owner", "", "New owner (email, name, 'me', or ID)")
	customerUpdateCmd.Flags().String("status", "", "New status name or ID")
	customerUpdateCmd.Flags().String("tier", "", "New tier name or ID")
	customerUpdateCmd.Flags().Int("revenue", 0, "New annual revenue")

	customerUpsertCmd.Flags().String("external-id", "", "External identifier to key on (required)")
	customerUpsertCmd.Flags().String("name", "", "Customer name (required)")
	customerUpsertCmd.Flags().String("tier-name", "", "Tier name (created if missing)")
	customerUpsertCmd.Flags().StringSlice("domains", nil, "Email domains (repeatable or comma-separated)")

	// customer need
	customerNeedListCmd.Flags().IntP("limit", "l", 50, "Maximum number of needs to fetch")
	customerNeedListCmd.Flags().String("customer", "", "Filter by customer name or ID")

	customerNeedCreateCmd.Flags().String("customer", "", "Customer name or ID (required)")
	customerNeedCreateCmd.Flags().String("body", "", "Need body text (required)")
	customerNeedCreateCmd.Flags().Float64("priority", 0, "Importance: 0 = not important, 1 = important")
	customerNeedCreateCmd.Flags().String("issue", "", "Link to an issue (identifier or ID); required unless --project is set")
	customerNeedCreateCmd.Flags().String("project", "", "Link to a project (name or ID); required unless --issue is set")

	customerNeedUpdateCmd.Flags().String("body", "", "New body text")
	customerNeedUpdateCmd.Flags().Float64("priority", 0, "Importance: 0 = not important, 1 = important")
	customerNeedUpdateCmd.Flags().String("issue", "", "Link to an issue (identifier or ID)")
	customerNeedUpdateCmd.Flags().String("project", "", "Link to a project (name or ID)")

	// customer status
	customerStatusCreateCmd.Flags().String("name", "", "Status name (required)")
	customerStatusCreateCmd.Flags().String("color", "", "Status color as a HEX string, e.g. #EB5757 (required)")
	customerStatusCreateCmd.Flags().StringP("description", "d", "", "Status description")

	customerStatusUpdateCmd.Flags().String("name", "", "New name")
	customerStatusUpdateCmd.Flags().String("color", "", "New color as a HEX string")
	customerStatusUpdateCmd.Flags().StringP("description", "d", "", "New description")

	// customer tier
	customerTierCreateCmd.Flags().String("name", "", "Tier name (required)")
	customerTierCreateCmd.Flags().String("color", "", "Tier color as a HEX string, e.g. #EB5757 (required)")
	customerTierCreateCmd.Flags().StringP("description", "d", "", "Tier description")

	customerTierUpdateCmd.Flags().String("name", "", "New name")
	customerTierUpdateCmd.Flags().String("color", "", "New color as a HEX string")
	customerTierUpdateCmd.Flags().StringP("description", "d", "", "New description")
}
