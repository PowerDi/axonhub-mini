import { HTMLAttributes } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { PasswordInput } from '@/components/password-input';
import { useInitializeSystem } from '@/features/auth/data/initialization';
import i18n from '@/lib/i18n';

type InitializationFormProps = HTMLAttributes<HTMLFormElement>;

// Create form schema factory to support i18n
const createFormSchema = (t: (key: string) => string) =>
  z.object({
    ownerEmail: z
      .string()
      .min(1, { message: t('initialization.form.validation.ownerEmailRequired') })
      .email({ message: t('initialization.form.validation.ownerEmailInvalid') }),
    ownerPassword: z
      .string()
      .min(1, {
        message: t('initialization.form.validation.ownerPasswordRequired'),
      })
      .min(8, {
        message: t('initialization.form.validation.ownerPasswordMinLength'),
      }),
    ownerFirstName: z.string().min(1, { message: t('initialization.form.validation.ownerFirstNameRequired') }),
    ownerLastName: z.string().min(1, { message: t('initialization.form.validation.ownerLastNameRequired') }),
    brandName: z.string().min(1, { message: t('initialization.form.validation.brandNameRequired') }),
  });

export function InitializationForm({ className, ...props }: InitializationFormProps) {
  const { t } = useTranslation();
  const initializeSystemMutation = useInitializeSystem();

  const formSchema = createFormSchema(t);
  type FormData = z.infer<typeof formSchema>;

  const form = useForm<FormData>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      ownerEmail: '',
      ownerPassword: '',
      ownerFirstName: '',
      ownerLastName: '',
      brandName: '',
    },
  });

  function onSubmit(data: FormData) {
    const input = {
      ownerEmail: data.ownerEmail,
      ownerPassword: data.ownerPassword,
      ownerFirstName: data.ownerFirstName,
      ownerLastName: data.ownerLastName,
      brandName: data.brandName,
      preferLanguage: i18n.language,
    };
    initializeSystemMutation.mutate(input);
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className={cn('grid gap-4', className)} {...props}>
        <FormField
          control={form.control}
          name='ownerFirstName'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('initialization.form.ownerFirstName')}</FormLabel>
              <FormControl>
                <Input placeholder={t('initialization.form.placeholders.ownerFirstName')} className='border-border !bg-white text-muted-foreground transition-all duration-300 placeholder:text-muted-foreground focus:border-border focus:!bg-white' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='ownerLastName'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('initialization.form.ownerLastName')}</FormLabel>
              <FormControl>
                <Input placeholder={t('initialization.form.placeholders.ownerLastName')} className='border-border !bg-white text-muted-foreground transition-all duration-300 placeholder:text-muted-foreground focus:border-border focus:!bg-white' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='ownerEmail'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('initialization.form.ownerEmail')}</FormLabel>
              <FormControl>
                <Input placeholder={t('initialization.form.placeholders.ownerEmail')} className='border-border !bg-white text-muted-foreground transition-all duration-300 placeholder:text-muted-foreground focus:border-border focus:!bg-white' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='ownerPassword'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('initialization.form.ownerPassword')}</FormLabel>
              <FormControl>
                <PasswordInput placeholder={t('initialization.form.placeholders.ownerPassword')} className='border-border bg-white text-muted-foreground backdrop-blur-sm transition-all duration-300 placeholder:text-muted-foreground focus:border-border focus:bg-white' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='brandName'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('initialization.form.brandName')}</FormLabel>
              <FormControl>
                <Input placeholder={t('initialization.form.placeholders.brandName')} className='border-border !bg-white text-muted-foreground transition-all duration-300 placeholder:text-muted-foreground focus:border-border focus:!bg-white' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button
          type='submit'
          className='bg-primary mt-6 w-full rounded-lg px-6 py-3 font-medium text-primary-foreground transition-all duration-300 hover:shadow-xl focus:ring-(--focus-ring) focus:ring-2 focus:ring-offset-2 disabled:opacity-50'
          disabled={initializeSystemMutation.isPending}
        >
          {initializeSystemMutation.isPending ? t('initialization.form.submitting') : t('initialization.form.submit')}
        </Button>
      </form>
    </Form>
  );
}
