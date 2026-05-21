import { Navigate, useParams } from "react-router";

export default function SourceDetailIndex() {
  const { name } = useParams<{ name: string }>();
  return <Navigate to={`/sources/${name}/schedule`} replace />;
}
