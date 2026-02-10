import { useState } from 'react'
import { IoArrowForwardSharp, IoCheckmarkCircleSharp } from 'react-icons/io5'
import { Button } from '@/components/ui/button'
import classNames from 'classnames'
import { useStartStore } from '../../store/useStartStore'
import { useGetConfigs } from '../../apis/config'
import { useLanguageStore } from '../../store/useLanguageStore'

export function SelectUserType() {

    const { type, setType, setStep } = useStartStore()
    const { data: config } = useGetConfigs()
    const { t } = useLanguageStore()

    return (
        <>
            <div className='mt-8 text-xl'>{t('choose_deployment')}</div>
            <div className='grid md:grid-cols-2 gap-4 mt-4 grid-cols-1'>
                <div className={classNames({
                    'p-4 bg-white rounded-lg cursor-pointer border-2 border-stone-200 relative': true,
                    '!border-primary': type === 'newbie'
                })} onClick={() => setType('newbie')}>
                    <div>🐦 {t('im_newbie')}</div>
                    <div className='text-sm mt-2 opacity-70'>{t('im_newbie_desc')}</div>
                    {type === 'newbie' && <IoCheckmarkCircleSharp className='text-primary absolute end-3 top-3 text-2xl' />}
                </div>
                <div className={classNames({
                    'p-4 bg-white rounded-lg cursor-pointer border-2 border-stone-200 relative': true,
                    '!border-primary': type === 'expert'
                })} onClick={() => setType('expert')}>
                    <div>🧓 {t('im_expert')}</div>
                    <div className='text-sm mt-2 opacity-70'>{t('im_expert_desc')}</div>
                    {type === 'expert' && <IoCheckmarkCircleSharp className='text-primary absolute end-3 top-3 text-2xl' />}
                </div>
            </div>
            <div className='mt-8 flex justify-between'>
                <div>
                    {!config?.ssl && <Button onClick={() => setStep('letsencrypt')} variant='outline'>
                        {t('back_to_cert')}
                    </Button>}
                </div>
                <Button className='cursor-pointer' onClick={() => setStep(`for-${type}`)}>{t('next_step')} <IoArrowForwardSharp /></Button>
            </div>
        </>
    )
}