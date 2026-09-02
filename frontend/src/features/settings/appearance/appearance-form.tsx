import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { showSubmittedData } from '@/utils/show-submitted-data';
import { useTheme } from '@/context/theme-context';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';

const appearanceFormSchema = z.object({
  theme: z.enum(['light', 'dark', 'system'], {
    error: 'Please select a theme.',
  }),
});

type AppearanceFormValues = z.infer<typeof appearanceFormSchema>;

export function AppearanceForm() {
  const { theme, setTheme } = useTheme();

  // This can come from your database or API.
  const defaultValues: Partial<AppearanceFormValues> = {
    theme,
  };

  const form = useForm<AppearanceFormValues>({
    resolver: zodResolver(appearanceFormSchema),
    defaultValues,
  });

  function onSubmit(data: AppearanceFormValues) {
    if (data.theme != theme) setTheme(data.theme);

    showSubmittedData(data);
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-8'>
        <FormField
          control={form.control}
          name='theme'
          render={({ field }) => (
            <FormItem className='space-y-1'>
              <FormLabel>Theme</FormLabel>
              <FormDescription>Select the theme for the dashboard.</FormDescription>
              <FormMessage />
              <RadioGroup onValueChange={field.onChange} defaultValue={field.value} className='grid max-w-md grid-cols-3 gap-8 pt-2'>
                <FormItem>
                  <FormLabel className='[&:has([data-state=checked])>div]:border-primary'>
                    <FormControl>
                      <RadioGroupItem value='light' className='sr-only' />
                    </FormControl>
                    <div className='border-muted hover:border-accent items-center rounded-md border-2 p-1 transition-colors'>
                      <div className='space-y-2 rounded-sm bg-[#f8f9fa] p-2'>
                        <div className='space-y-2 rounded-md bg-white p-2 shadow-sm'>
                          <div className='h-2 w-[80px] rounded-lg bg-[#e9ecef]' />
                          <div className='h-2 w-[100px] rounded-lg bg-[#e9ecef]' />
                        </div>
                        <div className='flex items-center space-x-2 rounded-md bg-white p-2 shadow-sm'>
                          <div className='bg-primary/20 h-4 w-4 rounded-full' />
                          <div className='h-2 w-[100px] rounded-lg bg-[#e9ecef]' />
                        </div>
                        <div className='flex items-center space-x-2 rounded-md bg-white p-2 shadow-sm'>
                          <div className='h-4 w-4 rounded-full bg-[#e9ecef]' />
                          <div className='h-2 w-[100px] rounded-lg bg-[#e9ecef]' />
                        </div>
                      </div>
                      <span className='block w-full p-2 text-center font-normal'>Light</span>
                    </div>
                  </FormLabel>
                </FormItem>
                <FormItem>
                  <FormLabel className='[&:has([data-state=checked])>div]:border-primary'>
                    <FormControl>
                      <RadioGroupItem value='dark' className='sr-only' />
                    </FormControl>
                    <div className='border-muted hover:border-accent items-center rounded-md border-2 p-1 transition-colors'>
                      <div className='space-y-2 rounded-sm bg-slate-950 p-2'>
                        <div className='space-y-2 rounded-md bg-slate-800 p-2 shadow-sm'>
                          <div className='h-2 w-[80px] rounded-lg bg-slate-600' />
                          <div className='h-2 w-[100px] rounded-lg bg-slate-600' />
                        </div>
                        <div className='flex items-center space-x-2 rounded-md bg-slate-800 p-2 shadow-sm'>
                          <div className='bg-primary/30 h-4 w-4 rounded-full' />
                          <div className='h-2 w-[100px] rounded-lg bg-slate-600' />
                        </div>
                        <div className='flex items-center space-x-2 rounded-md bg-slate-800 p-2 shadow-sm'>
                          <div className='h-4 w-4 rounded-full bg-slate-600' />
                          <div className='h-2 w-[100px] rounded-lg bg-slate-600' />
                        </div>
                      </div>
                      <span className='block w-full p-2 text-center font-normal'>Dark</span>
                    </div>
                  </FormLabel>
                </FormItem>
                <FormItem>
                  <FormLabel className='[&:has([data-state=checked])>div]:border-primary'>
                    <FormControl>
                      <RadioGroupItem value='system' className='sr-only' />
                    </FormControl>
                    <div className='border-muted hover:border-accent flex items-center justify-center rounded-md border-2 p-1 transition-colors'>
                      <div className='flex w-full overflow-hidden rounded-sm'>
                        <div className='w-1/2 space-y-2 bg-[#f8f9fa] p-2'>
                          <div className='h-2 w-[80px] rounded-lg bg-[#e9ecef]' />
                          <div className='h-2 w-[60px] rounded-lg bg-[#e9ecef]' />
                        </div>
                        <div className='w-1/2 space-y-2 bg-slate-950 p-2'>
                          <div className='h-2 w-[80px] rounded-lg bg-slate-600' />
                          <div className='h-2 w-[60px] rounded-lg bg-slate-600' />
                        </div>
                      </div>
                      <span className='block w-full p-2 text-center font-normal'>System</span>
                    </div>
                  </FormLabel>
                </FormItem>
              </RadioGroup>
            </FormItem>
          )}
        />

        <Button type='submit'>Update preferences</Button>
      </form>
    </Form>
  );
}
