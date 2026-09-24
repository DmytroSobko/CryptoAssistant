interface PlaceholderPageProps {
  title: string;
  description: string;
}

export function PlaceholderPage({ title, description }: PlaceholderPageProps) {
  return <section className="placeholder"><h1>{title}</h1><p>{description}</p></section>;
}

