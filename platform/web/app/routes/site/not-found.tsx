import { data } from "react-router";
import { publicHeaders } from "~/site/seo";

// Unknown paths under /es or /en: a real 404 rendered by the error boundary.
export function loader() {
  throw data(null, { status: 404, headers: publicHeaders });
}

export default function NotFound() {
  return null;
}
