package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

var (
	userRolesNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of user roles to retrieve. Minimum 1, maximum 1000.",
	}
)

var (
	userRoles = Flag{
		Name:       "Roles",
		LongForm:   "roles",
		ShortForm:  "r",
		Help:       "Roles to assign to a user.",
		IsRequired: true,
	}

	errNoRolesSelected = errors.New("required to select at least one role")
)

type userRolesInput struct {
	ID     string
	Number int
	Roles  []string
}

func userRolesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "Manage a user's roles",
		Long: "Manage a user's assigned roles. To learn more about roles and their behavior, read " +
			"[Role-based Access Control](https://auth0.com/docs/manage-users/access-control/rbac).",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showUserRolesCmd(cli))
	cmd.AddCommand(addUserRolesCmd(cli))
	cmd.AddCommand(removeUserRolesCmd(cli))

	return cmd
}

func showUserRolesCmd(cli *cli) *cobra.Command {
	var inputs userRolesInput

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a user's roles",
		Long:  "Display information about an existing user's assigned roles.",
		Example: `  auth0 users roles show
  auth0 users roles show <user-id>
  auth0 users roles show <user-id> --number 100
  auth0 users roles show <user-id> -n 100 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			list, err := getWithPagination(
				cmd.Context(),
				inputs.Number,
				func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
					userRoleList, err := cli.api.User.Roles(inputs.ID, opts...)
					if err != nil {
						return nil, false, err
					}

					var output []interface{}
					for _, userRole := range userRoleList.Roles {
						output = append(output, userRole)
					}

					return output, userRoleList.HasNext(), nil
				},
			)
			if err != nil {
				return fmt.Errorf("failed to find roles for user with ID %s: %w", inputs.ID, err)
			}

			var userRoles []*management.Role
			for _, item := range list {
				userRoles = append(userRoles, item.(*management.Role))
			}

			cli.renderer.UserRoleList(userRoles)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	userRolesNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	return cmd
}

func addUserRolesCmd(cli *cli) *cobra.Command {
	var inputs userRolesInput

	cmd := &cobra.Command{
		Use:     "assign",
		Aliases: []string{"add"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Assign roles to a user",
		Long:    "Assign existing roles to a user.",
		Example: `  auth0 users roles assign <user-id>
  auth0 users roles add <user-id> --roles <role-id1,role-id2>
  auth0 users roles add <user-id> -r "rol_1eKJp3jV04SiU04h,rol_2eKJp3jV04SiU04h" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if len(inputs.Roles) == 0 {
				if err := cli.pickUserRolesToAdd(&inputs); err != nil {
					return err
				}
			}

			var rolesToAssign []*management.Role
			for _, roleID := range inputs.Roles {
				rolesToAssign = append(rolesToAssign, &management.Role{
					ID: auth0.String(roleID),
				})
			}

			if err := ansi.Waiting(func() (err error) {
				return cli.api.User.AssignRoles(inputs.ID, rolesToAssign)
			}); err != nil {
				return fmt.Errorf("failed to assign roles for user with ID %s: %w", inputs.ID, err)
			}

			var userRoleList *management.RoleList
			if err := ansi.Waiting(func() (err error) {
				userRoleList, err = cli.api.User.Roles(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to find roles for user with ID %s: %w", inputs.ID, err)
			}

			cli.renderer.UserRoleList(userRoleList.Roles)

			return nil
		},
	}

	userRoles.RegisterStringSlice(cmd, &inputs.Roles, nil)
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func removeUserRolesCmd(cli *cli) *cobra.Command {
	var inputs userRolesInput

	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Remove roles from a user",
		Long:    "Remove existing roles from a user.",
		Example: `  auth0 users roles remove <user-id>
  auth0 users roles remove <user-id> --roles <role-id1,role-id2>
  auth0 users roles rm <user-id> -r "rol_1eKJp3jV04SiU04h,rol_2eKJp3jV04SiU04h" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if len(inputs.Roles) == 0 {
				if err := cli.pickUserRolesToRemove(&inputs); err != nil {
					return err
				}
			}

			var rolesToRemove []*management.Role
			for _, roleID := range inputs.Roles {
				rolesToRemove = append(rolesToRemove, &management.Role{
					ID: auth0.String(roleID),
				})
			}

			if err := ansi.Waiting(func() (err error) {
				return cli.api.User.RemoveRoles(inputs.ID, rolesToRemove)
			}); err != nil {
				return fmt.Errorf("failed to remove roles for user with ID %s: %w", inputs.ID, err)
			}

			var userRoleList *management.RoleList
			if err := ansi.Waiting(func() (err error) {
				userRoleList, err = cli.api.User.Roles(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to find roles for user with ID %s: %w", inputs.ID, err)
			}

			cli.renderer.UserRoleList(userRoleList.Roles)

			return nil
		},
	}

	userRoles.RegisterStringSlice(cmd, &inputs.Roles, nil)
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func (cli *cli) pickUserRolesToAdd(inputs *userRolesInput) error {
	var currentUserRoleList *management.RoleList
	if err := ansi.Waiting(func() (err error) {
		currentUserRoleList, err = cli.api.User.Roles(inputs.ID, management.PerPage(100))
		return err
	}); err != nil {
		return fmt.Errorf("failed to find the current roles for user with ID %s: %w", inputs.ID, err)
	}

	var roleList *management.RoleList
	if err := ansi.Waiting(func() (err error) {
		roleList, err = cli.api.Role.List()
		return err
	}); err != nil {
		return fmt.Errorf("failed to list all roles: %w", err)
	}

	if len(roleList.Roles) == len(currentUserRoleList.Roles) {
		return fmt.Errorf("the user with ID %q has all roles assigned already", inputs.ID)
	}

	const emptySpace = " "
	var options []string
	for _, role := range roleList.Roles {
		if !containsRole(currentUserRoleList.Roles, role.GetID()) {
			options = append(options, fmt.Sprintf("%s%s(Name: %s)", role.GetID(), emptySpace, role.GetName()))
		}
	}

	rolesPrompt := &survey.MultiSelect{
		Message: "Roles",
		Options: options,
	}

	var selectedRoles []string
	if err := survey.AskOne(rolesPrompt, &selectedRoles); err != nil {
		return err
	}

	for _, selectedRole := range selectedRoles {
		indexOfFirstEmptySpace := strings.Index(selectedRole, emptySpace)
		inputs.Roles = append(inputs.Roles, selectedRole[:indexOfFirstEmptySpace])
	}

	if len(inputs.Roles) == 0 {
		return errNoRolesSelected
	}

	return nil
}

func (cli *cli) pickUserRolesToRemove(inputs *userRolesInput) error {
	var currentUserRoleList *management.RoleList
	if err := ansi.Waiting(func() (err error) {
		currentUserRoleList, err = cli.api.User.Roles(inputs.ID)
		return err
	}); err != nil {
		return fmt.Errorf("failed to find the current roles for user with ID %s: %w", inputs.ID, err)
	}

	const emptySpace = " "
	var options []string
	for _, role := range currentUserRoleList.Roles {
		options = append(options, fmt.Sprintf("%s%s(Name: %s)", role.GetID(), emptySpace, role.GetName()))
	}

	rolesPrompt := &survey.MultiSelect{
		Message: "Roles",
		Options: options,
	}

	var selectedRoles []string
	if err := survey.AskOne(rolesPrompt, &selectedRoles); err != nil {
		return err
	}

	for _, selectedRole := range selectedRoles {
		indexOfFirstEmptySpace := strings.Index(selectedRole, emptySpace)
		inputs.Roles = append(inputs.Roles, selectedRole[:indexOfFirstEmptySpace])
	}

	if len(inputs.Roles) == 0 {
		return errNoRolesSelected
	}

	return nil
}

func containsRole(roles []*management.Role, roleID string) bool {
	for _, role := range roles {
		if role.GetID() == roleID {
			return true
		}
	}
	return false
}
