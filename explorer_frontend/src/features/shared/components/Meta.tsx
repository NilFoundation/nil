import { Helmet } from "react-helmet";

export type MetaProps = {
  title: string;
  description: string;
};

export const Meta = ({ title, description }: MetaProps) => {
  return (
    <Helmet>
      <title>{title}</title>
      <meta name="description" content={description} />
    </Helmet>
  );
};
