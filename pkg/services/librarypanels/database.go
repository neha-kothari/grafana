package librarypanels

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/grafana/pkg/util"

	"github.com/grafana/grafana/pkg/models"

	"github.com/grafana/grafana/pkg/services/sqlstore"
)

// createLibraryPanel adds a Library Panel.
func (lps *LibraryPanelService) createLibraryPanel(c *models.ReqContext, cmd createLibraryPanelCommand) (LibraryPanel, error) {
	libraryPanel := LibraryPanel{
		OrgID:    c.SignedInUser.OrgId,
		FolderID: cmd.FolderID,
		UID:      util.GenerateShortUID(),
		Name:     cmd.Name,
		Model:    cmd.Model,

		Created: time.Now(),
		Updated: time.Now(),

		CreatedBy: c.SignedInUser.UserId,
		UpdatedBy: c.SignedInUser.UserId,
	}
	err := lps.SQLStore.WithTransactionalDbSession(context.Background(), func(session *sqlstore.DBSession) error {
		if _, err := session.Insert(&libraryPanel); err != nil {
			if lps.SQLStore.Dialect.IsUniqueConstraintViolation(err) {
				return errLibraryPanelAlreadyExists
			}
			return err
		}
		return nil
	})

	return libraryPanel, err
}

// connectDashboard adds a connection between a Library Panel and a Dashboard.
func (lps *LibraryPanelService) connectDashboard(c *models.ReqContext, uid string, dashboardID int64) error {
	err := lps.SQLStore.WithTransactionalDbSession(context.Background(), func(session *sqlstore.DBSession) error {
		panel, err := getLibraryPanel(session, uid, c.SignedInUser.OrgId)
		if err != nil {
			return err
		}

		// TODO add check that dashboard exists

		libraryPanelDashboard := libraryPanelDashboard{
			DashboardID:    dashboardID,
			LibraryPanelID: panel.ID,
			Created:        time.Now(),
			CreatedBy:      c.SignedInUser.UserId,
		}
		if _, err := session.Insert(&libraryPanelDashboard); err != nil {
			if lps.SQLStore.Dialect.IsUniqueConstraintViolation(err) {
				return nil
			}
			return err
		}
		return nil
	})

	return err
}

// deleteLibraryPanel deletes a Library Panel.
func (lps *LibraryPanelService) deleteLibraryPanel(c *models.ReqContext, uid string) error {
	orgID := c.SignedInUser.OrgId
	return lps.SQLStore.WithTransactionalDbSession(context.Background(), func(session *sqlstore.DBSession) error {
		result, err := session.Exec("DELETE FROM library_panel WHERE uid=? and org_id=?", uid, orgID)
		if err != nil {
			return err
		}

		if rowsAffected, err := result.RowsAffected(); err != nil {
			return err
		} else if rowsAffected != 1 {
			return errLibraryPanelNotFound
		}

		return nil
	})
}

// disconnectDashboard deletes a connection between a Library Panel and a Dashboard.
func (lps *LibraryPanelService) disconnectDashboard(c *models.ReqContext, uid string, dashboardID int64) error {
	return lps.SQLStore.WithTransactionalDbSession(context.Background(), func(session *sqlstore.DBSession) error {
		panel, err := getLibraryPanel(session, uid, c.SignedInUser.OrgId)
		if err != nil {
			return err
		}

		result, err := session.Exec("DELETE FROM library_panel_dashboard WHERE librarypanel_id=? and dashboard_id=?", panel.ID, dashboardID)
		if err != nil {
			return err
		}

		if rowsAffected, err := result.RowsAffected(); err != nil {
			return err
		} else if rowsAffected != 1 {
			return errLibraryPanelDashboardNotFound
		}

		return nil
	})
}

func getLibraryPanel(session *sqlstore.DBSession, uid string, orgID int64) (LibraryPanel, error) {
	libraryPanels := make([]LibraryPanel, 0)
	session.Table("library_panel")
	session.Where("uid=? AND org_id=?", uid, orgID)
	err := session.Find(&libraryPanels)
	if err != nil {
		return LibraryPanel{}, err
	}
	if len(libraryPanels) == 0 {
		return LibraryPanel{}, errLibraryPanelNotFound
	}
	if len(libraryPanels) > 1 {
		return LibraryPanel{}, fmt.Errorf("found %d panels, while expecting at most one", len(libraryPanels))
	}

	return libraryPanels[0], nil
}

// getLibraryPanel gets a Library Panel.
func (lps *LibraryPanelService) getLibraryPanel(c *models.ReqContext, uid string) (LibraryPanel, error) {
	var libraryPanel LibraryPanel
	err := lps.SQLStore.WithDbSession(context.Background(), func(session *sqlstore.DBSession) error {
		var err error
		libraryPanel, err = getLibraryPanel(session, uid, c.SignedInUser.OrgId)
		return err
	})

	return libraryPanel, err
}

// getAllLibraryPanels gets all library panels.
func (lps *LibraryPanelService) getAllLibraryPanels(c *models.ReqContext) ([]LibraryPanel, error) {
	orgID := c.SignedInUser.OrgId
	libraryPanels := make([]LibraryPanel, 0)
	err := lps.SQLStore.WithDbSession(context.Background(), func(session *sqlstore.DBSession) error {
		err := session.SQL("SELECT * FROM library_panel WHERE org_id=?", orgID).Find(&libraryPanels)
		if err != nil {
			return err
		}

		return nil
	})

	return libraryPanels, err
}

// getConnectedDashboards gets all dashboards connected to a Library Panel.
func (lps *LibraryPanelService) getConnectedDashboards(c *models.ReqContext, uid string) ([]int64, error) {
	connectedDashboardIDs := make([]int64, 0)
	err := lps.SQLStore.WithDbSession(context.Background(), func(session *sqlstore.DBSession) error {
		panel, err := getLibraryPanel(session, uid, c.SignedInUser.OrgId)
		if err != nil {
			return err
		}

		var libraryPanelDashboards []libraryPanelDashboard
		session.Table("library_panel_dashboard")
		session.Where("librarypanel_id=?", panel.ID)
		err = session.Find(&libraryPanelDashboards)
		if err != nil {
			return err
		}

		for _, lpd := range libraryPanelDashboards {
			connectedDashboardIDs = append(connectedDashboardIDs, lpd.DashboardID)
		}

		return nil
	})

	return connectedDashboardIDs, err
}

// patchLibraryPanel updates a Library Panel.
func (lps *LibraryPanelService) patchLibraryPanel(c *models.ReqContext, cmd patchLibraryPanelCommand, uid string) (LibraryPanel, error) {
	var libraryPanel LibraryPanel
	err := lps.SQLStore.WithTransactionalDbSession(context.Background(), func(session *sqlstore.DBSession) error {
		panelInDB, err := getLibraryPanel(session, uid, c.SignedInUser.OrgId)
		if err != nil {
			return err
		}

		libraryPanel = LibraryPanel{
			ID:        panelInDB.ID,
			OrgID:     c.SignedInUser.OrgId,
			FolderID:  cmd.FolderID,
			UID:       uid,
			Name:      cmd.Name,
			Model:     cmd.Model,
			Created:   panelInDB.Created,
			CreatedBy: panelInDB.CreatedBy,
			Updated:   time.Now(),
			UpdatedBy: c.SignedInUser.UserId,
		}

		if cmd.FolderID == 0 {
			libraryPanel.FolderID = panelInDB.FolderID
		}
		if cmd.Name == "" {
			libraryPanel.Name = panelInDB.Name
		}
		if cmd.Model == nil {
			libraryPanel.Model = panelInDB.Model
		}

		if rowsAffected, err := session.ID(panelInDB.ID).Update(&libraryPanel); err != nil {
			if lps.SQLStore.Dialect.IsUniqueConstraintViolation(err) {
				return errLibraryPanelAlreadyExists
			}
			return err
		} else if rowsAffected != 1 {
			return errLibraryPanelNotFound
		}

		return nil
	})

	return libraryPanel, err
}
