import { useEffect } from 'react';
import { Type } from 'lucide-react';
import { IconCheck, IconMoon, IconSun } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { useFont } from '@/context/font-context';
import { useTheme } from '@/context/theme-context';
import { sansFonts, serifFonts, monoFonts, fontStacks, fontLabels } from '@/config/fonts';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

export function ThemeSwitch() {
  const { theme, setTheme } = useTheme();
  const { sansFont, serifFont, monoFont, setSansFont, setSerifFont, setMonoFont } = useFont();
  const { t } = useTranslation();

  /* Update theme-color meta tag when theme is updated */
  useEffect(() => {
    const themeColor = theme === 'dark' ? '#0b0b0c' : '#ffffff';
    const metaThemeColor = document.querySelector("meta[name='theme-color']");
    if (metaThemeColor) metaThemeColor.setAttribute('content', themeColor);
  }, [theme]);

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' size='icon' className='scale-95 rounded-full'>
          <IconSun className='size-[1.2rem] scale-100 rotate-0 transition-all dark:scale-0 dark:-rotate-90' />
          <IconMoon className='absolute size-[1.2rem] scale-0 rotate-90 transition-all dark:scale-100 dark:rotate-0' />
          <span className='sr-only'>{t('theme.toggle')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem onClick={() => setTheme('light')}>
          {t('theme.light')} <IconCheck size={14} className={cn('ml-auto', theme !== 'light' && 'hidden')} />
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme('dark')}>
          {t('theme.dark')}
          <IconCheck size={14} className={cn('ml-auto', theme !== 'dark' && 'hidden')} />
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme('system')}>
          {t('theme.system')}
          <IconCheck size={14} className={cn('ml-auto', theme !== 'system' && 'hidden')} />
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Type size={14} className='mr-2' />
            {t('theme.font')}
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <DropdownMenuSub>
              <DropdownMenuSubTrigger inset>{t('theme.font.sans')}</DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                {sansFonts.map((f) => (
                  <DropdownMenuItem key={f} onClick={() => setSansFont(f)} className='flex items-center justify-between'>
                    <span style={f === 'system' || f === 'theme' ? undefined : { fontFamily: fontStacks[f] }}>
                      {f === 'system' ? t('theme.font.followSystem') : f === 'theme' ? t('theme.font.followTheme') : fontLabels[f]}
                    </span>
                    <IconCheck size={14} className={cn('ml-auto', sansFont !== f && 'hidden')} />
                  </DropdownMenuItem>
                ))}
              </DropdownMenuSubContent>
            </DropdownMenuSub>
            <DropdownMenuSub>
              <DropdownMenuSubTrigger inset>{t('theme.font.serif')}</DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                {serifFonts.map((f) => (
                  <DropdownMenuItem key={f} onClick={() => setSerifFont(f)} className='flex items-center justify-between'>
                    <span style={f === 'system' || f === 'theme' ? undefined : { fontFamily: fontStacks[f] }}>
                      {f === 'system' ? t('theme.font.followSystem') : f === 'theme' ? t('theme.font.followTheme') : fontLabels[f]}
                    </span>
                    <IconCheck size={14} className={cn('ml-auto', serifFont !== f && 'hidden')} />
                  </DropdownMenuItem>
                ))}
              </DropdownMenuSubContent>
            </DropdownMenuSub>
            <DropdownMenuSub>
              <DropdownMenuSubTrigger inset>{t('theme.font.mono')}</DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                {monoFonts.map((f) => (
                  <DropdownMenuItem key={f} onClick={() => setMonoFont(f)} className='flex items-center justify-between'>
                    <span style={f === 'system' || f === 'theme' ? undefined : { fontFamily: fontStacks[f] }}>
                      {f === 'system' ? t('theme.font.followSystem') : f === 'theme' ? t('theme.font.followTheme') : fontLabels[f]}
                    </span>
                    <IconCheck size={14} className={cn('ml-auto', monoFont !== f && 'hidden')} />
                  </DropdownMenuItem>
                ))}
              </DropdownMenuSubContent>
            </DropdownMenuSub>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
