import type { AFSWorkspaceDetail, AFSWorkspaceView } from "../../foundation/types/afs";
import { FilesTab } from "./-files-tab";

type Props = {
  workspace: AFSWorkspaceDetail;
  browserView: AFSWorkspaceView;
  onBrowserViewChange: (view: AFSWorkspaceView) => void;
};

export function BrowseTab({
  workspace,
  browserView,
  onBrowserViewChange,
}: Props) {
  return (
    <FilesTab
      workspace={workspace}
      browserView={browserView}
      onBrowserViewChange={onBrowserViewChange}
    />
  );
}
